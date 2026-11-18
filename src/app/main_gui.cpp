#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <commctrl.h>
#include <dwmapi.h>
#include <unknwn.h>
#include <WebView2.h>
#include <shellapi.h>
#include <shobjidl.h>

#include <iostream>
#include <fstream>
#include <string>
#include <vector>
#include <mutex>
#include <atomic>
#include <functional>
#include <cstdarg>

#define WM_POST_JSON (WM_APP + 1)
#define IDI_ICON1 101
#define IDR_INDEX_HTML 201
#define IDR_ENGINE_DLL 202
#define IDR_WV2_LOADER 203
#define IDR_APP_WASM   204

// Go Engine DLL function pointer types
typedef char* (*GetSubnetsJSONFn)();
typedef int (*StartScanEngineFn)(const char* cidr, const char* ports, int concurrency, int scanPorts, void (*cb)(const char*));
typedef void (*StopScanEngineFn)();
typedef void (*FreeCStringFn)(char*);

static HMODULE g_hEngineDll = nullptr;
static GetSubnetsJSONFn g_pfnGetSubnets = nullptr;
static StartScanEngineFn g_pfnStartScan = nullptr;
static StopScanEngineFn g_pfnStopScan = nullptr;
static FreeCStringFn g_pfnFreeCString = nullptr;

static FILE* g_log = nullptr;
static std::wstring g_logPath;
static std::wstring g_userDataFolder;

static void LOG(const char* fmt, ...) {
    if (!g_log) {
        if (g_logPath.empty()) {
            wchar_t exePath[MAX_PATH] = {0};
            GetModuleFileNameW(NULL, exePath, MAX_PATH);
            g_logPath = exePath;
            size_t slash = g_logPath.find_last_of(L"\\/");
            if (slash != std::wstring::npos) g_logPath.resize(slash + 1);
            g_logPath += L"QuickNetScan_debug.log";
        }
        char path[MAX_PATH * 3] = {0};
        WideCharToMultiByte(CP_UTF8, 0, g_logPath.c_str(), -1, path, sizeof(path), NULL, NULL);
        g_log = fopen(path, "w");
    }
    if (g_log) {
        va_list args;
        va_start(args, fmt);
        vfprintf(g_log, fmt, args);
        fprintf(g_log, "\n");
        fflush(g_log);
        va_end(args);
    }
}

static HWND g_hWnd = nullptr;
static ICoreWebView2* g_pWebView = nullptr;
static ICoreWebView2Controller* g_pController = nullptr;

// WebView2 loader typedef
typedef HRESULT (STDAPICALLTYPE *CreateEnvFn)(
    PCWSTR browserExecutableFolder,
    PCWSTR userDataFolder,
    ICoreWebView2EnvironmentOptions* environmentOptions,
    ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler* environment_created_handler
);

class EnvCompletedHandler : public ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler {
    std::function<HRESULT(HRESULT, ICoreWebView2Environment*)> m_fn;
    std::atomic<ULONG> m_ref{ 1 };
public:
    EnvCompletedHandler(std::function<HRESULT(HRESULT, ICoreWebView2Environment*)> fn) : m_fn(fn) {}
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void** ppv) override {
        if (riid == IID_IUnknown || riid == IID_ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler) {
            *ppv = static_cast<ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler*>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++m_ref; }
    ULONG STDMETHODCALLTYPE Release() override {
        ULONG r = --m_ref;
        if (r == 0) delete this;
        return r;
    }
    HRESULT STDMETHODCALLTYPE Invoke(HRESULT result, ICoreWebView2Environment* env) override {
        return m_fn(result, env);
    }
};

class ControllerCompletedHandler : public ICoreWebView2CreateCoreWebView2ControllerCompletedHandler {
    std::function<HRESULT(HRESULT, ICoreWebView2Controller*)> m_fn;
    std::atomic<ULONG> m_ref{ 1 };
public:
    ControllerCompletedHandler(std::function<HRESULT(HRESULT, ICoreWebView2Controller*)> fn) : m_fn(fn) {}
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void** ppv) override {
        if (riid == IID_IUnknown || riid == IID_ICoreWebView2CreateCoreWebView2ControllerCompletedHandler) {
            *ppv = static_cast<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler*>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++m_ref; }
    ULONG STDMETHODCALLTYPE Release() override {
        ULONG r = --m_ref;
        if (r == 0) delete this;
        return r;
    }
    HRESULT STDMETHODCALLTYPE Invoke(HRESULT result, ICoreWebView2Controller* controller) override {
        return m_fn(result, controller);
    }
};

class WebMessageReceivedHandler : public ICoreWebView2WebMessageReceivedEventHandler {
    std::function<HRESULT(ICoreWebView2*, ICoreWebView2WebMessageReceivedEventArgs*)> m_fn;
    std::atomic<ULONG> m_ref{ 1 };
public:
    WebMessageReceivedHandler(std::function<HRESULT(ICoreWebView2*, ICoreWebView2WebMessageReceivedEventArgs*)> fn) : m_fn(fn) {}
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void** ppv) override {
        if (riid == IID_IUnknown || riid == IID_ICoreWebView2WebMessageReceivedEventHandler) {
            *ppv = static_cast<ICoreWebView2WebMessageReceivedEventHandler*>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++m_ref; }
    ULONG STDMETHODCALLTYPE Release() override {
        ULONG r = --m_ref;
        if (r == 0) delete this;
        return r;
    }
    HRESULT STDMETHODCALLTYPE Invoke(ICoreWebView2* sender, ICoreWebView2WebMessageReceivedEventArgs* args) override {
        return m_fn(sender, args);
    }
};

static std::wstring Utf8ToWide(const std::string& str) {
    if (str.empty()) return L"";
    int wlen = MultiByteToWideChar(CP_UTF8, 0, str.data(), (int)str.size(), NULL, 0);
    if (wlen <= 0) return L"";
    std::wstring wstr(wlen, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, str.data(), (int)str.size(), &wstr[0], wlen);
    return wstr;
}

static std::string WideToUtf8(const std::wstring& wstr) {
    if (wstr.empty()) return "";
    int len = WideCharToMultiByte(CP_UTF8, 0, wstr.data(), (int)wstr.size(), NULL, 0, NULL, NULL);
    if (len <= 0) return "";
    std::string str(len, '\0');
    WideCharToMultiByte(CP_UTF8, 0, wstr.data(), (int)wstr.size(), &str[0], len, NULL, NULL);
    return str;
}

static std::wstring JsonEscape(const std::wstring& value) {
    std::wstring escaped;
    escaped.reserve(value.size());
    for (wchar_t ch : value) {
        switch (ch) {
            case L'\\': escaped += L"\\\\"; break;
            case L'"':  escaped += L"\\\""; break;
            case L'\n': escaped += L"\\n";  break;
            case L'\r': escaped += L"\\r";  break;
            case L'\t': escaped += L"\\t";  break;
            default:    escaped += ch;      break;
        }
    }
    return escaped;
}

static void PostJsonToUI(const std::wstring& jsonStr) {
    if (g_hWnd) {
        wchar_t* msgCopy = _wcsdup(jsonStr.c_str());
        if (!PostMessageW(g_hWnd, WM_POST_JSON, 0, (LPARAM)msgCopy)) {
            free(msgCopy);
        }
    }
}

// Callback invoked by Go Engine Goroutines
static void OnProgressFromGo(const char* jsonUpdate) {
    if (!jsonUpdate) return;
    std::wstring wstr = Utf8ToWide(jsonUpdate);
    if (!wstr.empty()) {
        std::wstring wrapped = L"{\"cmd\":\"progress\",\"data\":" + wstr + L"}";
        PostJsonToUI(wrapped);
    }
}

// Load embedded UTF-8 resource as UTF-16 wstring for NavigateToString
static std::wstring LoadEmbeddedHtmlString() {
    HRSRC hRes = FindResourceW(NULL, MAKEINTRESOURCEW(IDR_INDEX_HTML), (LPCWSTR)RT_RCDATA);
    if (!hRes) return L"";
    HGLOBAL hGlobal = LoadResource(NULL, hRes);
    if (!hGlobal) return L"";
    DWORD size = SizeofResource(NULL, hRes);
    const char* pData = (const char*)LockResource(hGlobal);
    if (!pData || size == 0) return L"";
    int wlen = MultiByteToWideChar(CP_UTF8, 0, pData, (int)size, NULL, 0);
    if (wlen <= 0) return L"";
    std::wstring wstr(wlen, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, pData, (int)size, &wstr[0], wlen);
    return wstr;
}

// Extract an embedded RCDATA resource to a file on disk
static bool ExtractResourceToFile(int resourceId, const std::wstring& destPath) {
    HRSRC hRes = FindResourceW(NULL, MAKEINTRESOURCEW(resourceId), (LPCWSTR)RT_RCDATA);
    if (!hRes) return false;
    HGLOBAL hGlobal = LoadResource(NULL, hRes);
    if (!hGlobal) return false;
    DWORD size = SizeofResource(NULL, hRes);
    const void* pData = LockResource(hGlobal);
    if (!pData || size == 0) return false;

    HANDLE hFile = CreateFileW(destPath.c_str(), GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) return false;
    DWORD written = 0;
    BOOL ok = WriteFile(hFile, pData, size, &written, NULL);
    CloseHandle(hFile);
    return (ok && written == size);
}

static bool LoadGoEngine(const std::wstring& exeDir, const std::wstring& fallbackDir) {
    std::wstring dllPath = exeDir + L"engine_x64.dll";
    g_hEngineDll = LoadLibraryW(dllPath.c_str());
    if (!g_hEngineDll) {
        dllPath = exeDir + L"core\\native\\engine_x64.dll";
        g_hEngineDll = LoadLibraryW(dllPath.c_str());
    }
    if (!g_hEngineDll && !fallbackDir.empty()) {
        std::wstring extractedDll = fallbackDir + L"\\engine_x64.dll";
        if (ExtractResourceToFile(IDR_ENGINE_DLL, extractedDll)) {
            LOG("[Engine] Extracted embedded engine_x64.dll to %ls", extractedDll.c_str());
            g_hEngineDll = LoadLibraryW(extractedDll.c_str());
        }
    }
    if (!g_hEngineDll) return false;

    g_pfnGetSubnets = (GetSubnetsJSONFn)GetProcAddress(g_hEngineDll, "GetSubnetsJSON");
    g_pfnStartScan = (StartScanEngineFn)GetProcAddress(g_hEngineDll, "StartScanEngine");
    g_pfnStopScan = (StopScanEngineFn)GetProcAddress(g_hEngineDll, "StopScanEngine");
    g_pfnFreeCString = (FreeCStringFn)GetProcAddress(g_hEngineDll, "FreeCString");

    return (g_pfnGetSubnets && g_pfnStartScan && g_pfnStopScan);
}

static bool ExtractJsonString(const std::wstring& json, const std::wstring& key, std::wstring& out) {
    std::wstring needle = L"\"" + key + L"\"";
    size_t pos = json.find(needle);
    if (pos == std::wstring::npos) return false;
    pos = json.find(L':', pos + needle.size());
    if (pos == std::wstring::npos) return false;
    pos = json.find_first_not_of(L" \t\r\n", pos + 1);
    if (pos == std::wstring::npos || json[pos] != L'"') return false;
    ++pos;

    std::wstring result;
    while (pos < json.size() && json[pos] != L'"') {
        wchar_t ch = json[pos];
        if (ch == L'\\' && pos + 1 < json.size()) {
            wchar_t next = json[pos + 1];
            switch (next) {
                case L'"':  result += L'"';  pos += 2; break;
                case L'\\': result += L'\\'; pos += 2; break;
                case L'/':  result += L'/';  pos += 2; break;
                case L'n':  result += L'\n'; pos += 2; break;
                case L'r':  result += L'\r'; pos += 2; break;
                case L't':  result += L'\t'; pos += 2; break;
                case L'u':
                    if (pos + 6 <= json.size()) {
                        wchar_t code = (wchar_t)wcstol(json.substr(pos + 2, 4).c_str(), nullptr, 16);
                        result += code;
                        pos += 6;
                    } else {
                        result += ch; ++pos;
                    }
                    break;
                default: result += next; pos += 2; break;
            }
        } else {
            result += ch;
            ++pos;
        }
    }
    out = result;
    return true;
}

static int ExtractJsonInt(const std::wstring& json, const std::wstring& key, int defaultVal) {
    std::wstring needle = L"\"" + key + L"\"";
    size_t pos = json.find(needle);
    if (pos == std::wstring::npos) return defaultVal;
    pos = json.find(L':', pos + needle.size());
    if (pos == std::wstring::npos) return defaultVal;
    pos = json.find_first_not_of(L" \t\r\n", pos + 1);
    if (pos == std::wstring::npos) return defaultVal;
    try {
        return std::stoi(json.substr(pos));
    } catch (...) {
        return defaultVal;
    }
}

static bool ExtractJsonBool(const std::wstring& json, const std::wstring& key, bool defaultVal) {
    std::wstring needle = L"\"" + key + L"\"";
    size_t pos = json.find(needle);
    if (pos == std::wstring::npos) return defaultVal;
    pos = json.find(L':', pos + needle.size());
    if (pos == std::wstring::npos) return defaultVal;
    pos = json.find_first_not_of(L" \t\r\n", pos + 1);
    if (pos == std::wstring::npos) return defaultVal;
    if (json.compare(pos, 4, L"true") == 0) return true;
    if (json.compare(pos, 5, L"false") == 0) return false;
    return defaultVal;
}

static void HandleExportCsv(const std::wstring& csvContent) {
    IFileSaveDialog* pSave = nullptr;
    HRESULT hr = CoCreateInstance(CLSID_FileSaveDialog, NULL, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&pSave));
    if (FAILED(hr) || !pSave) return;

    COMDLG_FILTERSPEC rgSpec[] = {
        { L"CSV Files (*.csv)", L"*.csv" },
        { L"All Files (*.*)", L"*.*" }
    };
    pSave->SetFileTypes(ARRAYSIZE(rgSpec), rgSpec);
    pSave->SetFileTypeIndex(1);
    pSave->SetDefaultExtension(L"csv");
    pSave->SetFileName(L"quicknetscan_results.csv");

    std::wstring savedPath;
    bool ok = false;
    if (SUCCEEDED(pSave->Show(g_hWnd))) {
        IShellItem* pItem = nullptr;
        if (SUCCEEDED(pSave->GetResult(&pItem)) && pItem) {
            PWSTR pszFilePath = nullptr;
            if (SUCCEEDED(pItem->GetDisplayName(SIGDN_FILESYSPATH, &pszFilePath)) && pszFilePath) {
                savedPath = pszFilePath;
                CoTaskMemFree(pszFilePath);
            }
            pItem->Release();
        }
    }
    pSave->Release();

    if (savedPath.empty()) return;

    std::string utf8 = "\xEF\xBB\xBF" + WideToUtf8(csvContent);
    HANDLE hFile = CreateFileW(savedPath.c_str(), GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile != INVALID_HANDLE_VALUE) {
        DWORD written = 0;
        BOOL wOk = WriteFile(hFile, utf8.data(), (DWORD)utf8.size(), &written, NULL);
        ok = (wOk && written == (DWORD)utf8.size());
        CloseHandle(hFile);
    }

    std::wstring resp = L"{\"cmd\":\"export_result\",\"success\":" + std::wstring(ok ? L"true" : L"false")
                      + L",\"path\":\"" + JsonEscape(savedPath) + L"\"}";
    PostJsonToUI(resp);
}

static void HandleWebMessage(const std::wstring& msgW) {
    if (msgW.empty()) return;
    try {
        LOG("[MSG] HandleWebMessage received: %ls", msgW.c_str());

        std::wstring cmd;
        if (!ExtractJsonString(msgW, L"cmd", cmd)) {
            ExtractJsonString(msgW, L"type", cmd);
        }
        LOG("[MSG] Parsed cmd: %ls", cmd.c_str());

        if (cmd == L"ping") {
            PostJsonToUI(L"{\"type\":\"pong\",\"cmd\":\"pong\"}");
        } else if (cmd == L"get_subnets") {
            if (g_pfnGetSubnets) {
                char* pJson = g_pfnGetSubnets();
                LOG("[MSG] g_pfnGetSubnets returned: %s", pJson ? pJson : "NULL");
                if (pJson) {
                    std::wstring wsubnets = Utf8ToWide(pJson);
                    if (!wsubnets.empty()) {
                        std::wstring resp = L"{\"cmd\":\"subnets_result\",\"subnets\":" + wsubnets + L"}";
                        PostJsonToUI(resp);
                    }
                    if (g_pfnFreeCString) g_pfnFreeCString(pJson);
                }
            }
        } else if (cmd == L"start_scan") {
            std::wstring cidrW, portsW;
            ExtractJsonString(msgW, L"target_cidr", cidrW);
            ExtractJsonString(msgW, L"ports", portsW);
            int concurrency = ExtractJsonInt(msgW, L"concurrency", 256);
            bool scanPorts = ExtractJsonBool(msgW, L"scan_ports", true);

            std::string cidr = WideToUtf8(cidrW);
            std::string ports = WideToUtf8(portsW);

            LOG("[MSG] start_scan on %s with %d threads (scanPorts=%d)", cidr.c_str(), concurrency, scanPorts ? 1 : 0);
            if (g_pfnStartScan) {
                int totalTargets = g_pfnStartScan(cidr.c_str(), ports.c_str(), concurrency, scanPorts ? 1 : 0, OnProgressFromGo);
                LOG("[MSG] StartScanEngine returned totalTargets=%d", totalTargets);
                if (totalTargets < 0) {
                    std::wstring werr = L"{\"cmd\":\"scan_error\",\"error\":\"" + JsonEscape(cidrW) + L"\"}";
                    PostJsonToUI(werr);
                } else {
                    std::wstring wstarted = L"{\"cmd\":\"scan_started\",\"total_targets\":" + std::to_wstring(totalTargets) + L"}";
                    PostJsonToUI(wstarted);
                }
            } else {
                PostJsonToUI(L"{\"cmd\":\"scan_error\",\"error\":\"Engine DLL not loaded\"}");
            }
        } else if (cmd == L"stop_scan") {
            LOG("[MSG] stop_scan requested");
            if (g_pfnStopScan) {
                g_pfnStopScan();
            }
        } else if (cmd == L"export_csv") {
            std::wstring csvW;
            if (ExtractJsonString(msgW, L"csv", csvW)) {
                HandleExportCsv(csvW);
            }
        }
    } catch (const std::exception& ex) {
        LOG("[EXC] HandleWebMessage exception: %s", ex.what());
    } catch (...) {
        LOG("[EXC] HandleWebMessage unknown exception");
    }
}

static LRESULT CALLBACK WndProc(HWND hWnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    switch (msg) {
    case WM_SIZE:
        if (g_pController) {
            RECT bounds;
            GetClientRect(hWnd, &bounds);
            g_pController->put_Bounds(bounds);
        }
        return 0;

    case WM_POST_JSON: {
        wchar_t* pJson = (wchar_t*)lParam;
        if (pJson) {
            if (g_pWebView) {
                HRESULT hrPost = g_pWebView->PostWebMessageAsJson(pJson);
                if (FAILED(hrPost)) {
                    LOG("[ERR] PostWebMessageAsJson failed: 0x%08X", (unsigned)hrPost);
                }
            }
            free(pJson);
        }
        return 0;
    }

    case WM_DESTROY:
        if (g_pfnStopScan) g_pfnStopScan();
        if (g_pController) {
            g_pController->Close();
            g_pController->Release();
            g_pController = nullptr;
        }
        if (g_pWebView) {
            g_pWebView->Release();
            g_pWebView = nullptr;
        }
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hWnd, msg, wParam, lParam);
}

int WINAPI WinMain(HINSTANCE hInstance, HINSTANCE, LPSTR, int) {
    LOG("[0] Program start");
    CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    OleInitialize(NULL);
    SetProcessDPIAware();

    wchar_t exeDir[MAX_PATH] = {0};
    GetModuleFileNameW(NULL, exeDir, MAX_PATH);
    wchar_t* pSep = wcsrchr(exeDir, L'\\');
    if (pSep) *(pSep + 1) = L'\0';

    // Initialize WebView2 with app-specific user data folder
    wchar_t tempPath[MAX_PATH] = {0};
    GetTempPathW(MAX_PATH, tempPath);
    g_userDataFolder = std::wstring(tempPath) + L"QuickNetScan_WVData";
    CreateDirectoryW(g_userDataFolder.c_str(), NULL);
    LOG("[userDataFolder] %ls", g_userDataFolder.c_str());

    LOG("[2] Loading Go Engine...");
    bool engineOk = LoadGoEngine(exeDir, g_userDataFolder);
    LOG("[3] LoadGoEngine result: %d", engineOk);

    WNDCLASSEXW wc = { sizeof(wc) };
    wc.style = CS_HREDRAW | CS_VREDRAW;
    wc.lpfnWndProc = WndProc;
    wc.hInstance = hInstance;
    wc.lpszClassName = L"QuickNetScanWindowClass";
    wc.hIcon = LoadIconW(hInstance, MAKEINTRESOURCEW(IDI_ICON1));
    wc.hCursor = LoadCursorW(NULL, (LPCWSTR)IDC_ARROW);
    wc.hbrBackground = (HBRUSH)GetStockObject(BLACK_BRUSH);
    RegisterClassExW(&wc);

    LOG("[4] Creating Window...");
    g_hWnd = CreateWindowExW(
        0, wc.lpszClassName, L"QuickNetScan",
        WS_OVERLAPPEDWINDOW,
        CW_USEDEFAULT, CW_USEDEFAULT, 1080, 750,
        NULL, NULL, hInstance, NULL
    );

    if (!g_hWnd) {
        LOG("[ERR] CreateWindowExW failed: %lu", GetLastError());
        return 1;
    }
    LOG("[5] Window created: %p", (void*)g_hWnd);

    // Dark theme title bar
    BOOL dark = TRUE;
    DwmSetWindowAttribute(g_hWnd, 20 /* DWMWA_USE_IMMERSIVE_DARK_MODE */, &dark, sizeof(dark));

    ShowWindow(g_hWnd, SW_SHOWNORMAL);
    UpdateWindow(g_hWnd);
    SetForegroundWindow(g_hWnd);

    // Prepare embedded HTML in memory and app.wasm on disk
    std::wstring htmlContent = LoadEmbeddedHtmlString();
    std::wstring uiHostDir = g_userDataFolder + L"\\ui";
    CreateDirectoryW(uiHostDir.c_str(), NULL);
    std::wstring embeddedWasmFile = uiHostDir + L"\\app.wasm";
    bool wasmExtracted = ExtractResourceToFile(IDR_APP_WASM, embeddedWasmFile);
    LOG("[5-1] Loaded embedded HTML (%zu chars) and extracted WASM (%d) to %ls", htmlContent.size(), wasmExtracted, uiHostDir.c_str());

    std::wstring loaderPath = std::wstring(exeDir) + L"WebView2Loader.dll";
    LOG("[6] Loading WebView2Loader: %ls", loaderPath.c_str());
    HMODULE hLoader = LoadLibraryW(loaderPath.c_str());
    if (!hLoader) hLoader = LoadLibraryW(L"WebView2Loader.dll");
    if (!hLoader) {
        std::wstring extractedLoader = g_userDataFolder + L"\\WebView2Loader.dll";
        if (ExtractResourceToFile(IDR_WV2_LOADER, extractedLoader)) {
            LOG("[6-1] Extracted embedded WebView2Loader.dll to %ls", extractedLoader.c_str());
            hLoader = LoadLibraryW(extractedLoader.c_str());
        }
    }
    LOG("[7] hLoader: %p", (void*)hLoader);

    CreateEnvFn createEnv = hLoader ? (CreateEnvFn)GetProcAddress(hLoader, "CreateCoreWebView2EnvironmentWithOptions") : nullptr;
    LOG("[8] createEnv: %p", (void*)createEnv);

    auto onEnvCreated = [uiHostDir, htmlContent](HRESULT hr, ICoreWebView2Environment* env) -> HRESULT {
        LOG("[9] onEnvCreated: hr=0x%08X, env=%p", (unsigned)hr, (void*)env);
        if (FAILED(hr) || !env) return hr;
        env->CreateCoreWebView2Controller(g_hWnd, new ControllerCompletedHandler([uiHostDir, htmlContent](HRESULT hr2, ICoreWebView2Controller* controller) -> HRESULT {
            LOG("[10] onControllerCreated: hr=0x%08X, ctrl=%p", (unsigned)hr2, (void*)controller);
            if (FAILED(hr2) || !controller) {
                wchar_t errBuf[256];
                swprintf_s(errBuf, L"WebView2 Controller creation failed: 0x%08X", (unsigned)hr2);
                MessageBoxW(g_hWnd, errBuf, L"QuickNetScan Error", MB_ICONERROR);
                return hr2;
            }
            g_pController = controller;
            controller->AddRef();

            RECT bounds;
            GetClientRect(g_hWnd, &bounds);
            controller->put_Bounds(bounds);
            controller->put_IsVisible(TRUE);
            LOG("[10-1] Bounds set");

            controller->get_CoreWebView2(&g_pWebView);
            if (!g_pWebView) {
                LOG("[ERR] g_pWebView is NULL");
                return E_FAIL;
            }
            LOG("[10-2] g_pWebView acquired: %p", (void*)g_pWebView);

            ICoreWebView2Settings* settings = nullptr;
            g_pWebView->get_Settings(&settings);
            if (settings) {
                settings->put_AreDefaultContextMenusEnabled(FALSE);
                settings->put_IsZoomControlEnabled(FALSE);
                settings->put_AreDevToolsEnabled(TRUE);
                settings->Release();
            }
            LOG("[10-3] Settings configured");

            EventRegistrationToken token;
            g_pWebView->add_WebMessageReceived(new WebMessageReceivedHandler([](ICoreWebView2*, ICoreWebView2WebMessageReceivedEventArgs* args) -> HRESULT {
                LPWSTR rawMsg = nullptr;
                args->get_WebMessageAsJson(&rawMsg);
                if (rawMsg) {
                    HandleWebMessage(rawMsg);
                    CoTaskMemFree(rawMsg);
                }
                return S_OK;
            }), &token);
            LOG("[10-4] WebMessage handler registered");

            ICoreWebView2_3* webview3 = nullptr;
            HRESULT hrQI = g_pWebView->QueryInterface(IID_ICoreWebView2_3, (void**)&webview3);
            if (SUCCEEDED(hrQI) && webview3) {
                webview3->SetVirtualHostNameToFolderMapping(
                    L"quicknetscan.local",
                    uiHostDir.c_str(),
                    COREWEBVIEW2_HOST_RESOURCE_ACCESS_KIND_ALLOW
                );
                webview3->Release();
            }

            HRESULT hrNav = g_pWebView->NavigateToString(htmlContent.c_str());
            LOG("[10-8] NavigateToString: hr=0x%08X (%zu chars)", (unsigned)hrNav, htmlContent.size());
            LOG("[10-10] Done setup");
            return S_OK;
        }));
        return S_OK;
    };

    if (createEnv) {
        LOG("[11] Calling createEnv...");
        HRESULT hrEnv = createEnv(nullptr, g_userDataFolder.c_str(), nullptr, new EnvCompletedHandler(onEnvCreated));
        LOG("[12] createEnv returned: 0x%08X", (unsigned)hrEnv);
    } else {
        LOG("[ERR] createEnv is NULL!");
    }

    LOG("[13] Entering message loop...");
    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0)) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    LOG("[14] Exited message loop: wParam=%d", (int)msg.wParam);

    if (g_hEngineDll) FreeLibrary(g_hEngineDll);
    if (hLoader) FreeLibrary(hLoader);
    if (g_log) fclose(g_log);
    return (int)msg.wParam;
}

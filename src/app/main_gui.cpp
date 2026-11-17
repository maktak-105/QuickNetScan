#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <commctrl.h>
#include <dwmapi.h>
#include <unknwn.h>
#include <WebView2.h>
#include <shellapi.h>
#include <shobjidl.h>

#include <iostream>
#include <string>
#include <vector>
#include <mutex>
#include <atomic>
#include <functional>

#define WM_POST_JSON (WM_APP + 1)
#define IDI_ICON1 101
#define IDR_INDEX_HTML 201

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
static void LOG(const char* fmt, ...) {
    if (!g_log) {
        g_log = fopen("C:\\Users\\0120025-Z100\\source\\project\\Quicknetscan\\dist\\debug.log", "w");
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
static std::mutex g_postLock;

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

static void PostJsonToUI(const std::wstring& jsonStr) {
    if (g_hWnd) {
        wchar_t* msgCopy = _wcsdup(jsonStr.c_str());
        PostMessageW(g_hWnd, WM_POST_JSON, 0, (LPARAM)msgCopy);
    }
}

// Callback invoked by Go Engine Goroutines
static void OnProgressFromGo(const char* jsonUpdate) {
    if (!jsonUpdate) return;
    int len = MultiByteToWideChar(CP_UTF8, 0, jsonUpdate, -1, NULL, 0);
    if (len > 0) {
        std::wstring wstr(len, 0);
        MultiByteToWideChar(CP_UTF8, 0, jsonUpdate, -1, &wstr[0], len);
        wstr.resize(len - 1);
        std::wstring wrapped = L"{\"cmd\":\"progress\",\"data\":" + wstr + L"}";
        PostJsonToUI(wrapped);
    }
}

static bool LoadGoEngine() {
    wchar_t exePath[MAX_PATH];
    GetModuleFileNameW(NULL, exePath, MAX_PATH);
    wchar_t* pSlash = wcsrchr(exePath, L'\\');
    if (pSlash) *(pSlash + 1) = L'\0';
    std::wstring dllPath = std::wstring(exePath) + L"engine_x64.dll";

    g_hEngineDll = LoadLibraryW(dllPath.c_str());
    if (!g_hEngineDll) {
        // Try core/native/engine_x64.dll
        dllPath = std::wstring(exePath) + L"core\\native\\engine_x64.dll";
        g_hEngineDll = LoadLibraryW(dllPath.c_str());
    }
    if (!g_hEngineDll) return false;

    g_pfnGetSubnets = (GetSubnetsJSONFn)GetProcAddress(g_hEngineDll, "GetSubnetsJSON");
    g_pfnStartScan = (StartScanEngineFn)GetProcAddress(g_hEngineDll, "StartScanEngine");
    g_pfnStopScan = (StopScanEngineFn)GetProcAddress(g_hEngineDll, "StopScanEngine");
    g_pfnFreeCString = (FreeCStringFn)GetProcAddress(g_hEngineDll, "FreeCString");

    return (g_pfnGetSubnets && g_pfnStartScan && g_pfnStopScan);
}

static std::string GetStringProp(const std::string& json, const std::string& key) {
    std::string search = "\"" + key + "\":\"";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return "";
    size_t start = pos + search.length();
    size_t end = json.find("\"", start);
    if (end == std::string::npos) return "";
    return json.substr(start, end - start);
}

static int GetIntProp(const std::string& json, const std::string& key, int defaultVal) {
    std::string search = "\"" + key + "\":";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return defaultVal;
    size_t start = pos + search.length();
    while (start < json.length() && (json[start] == ' ' || json[start] == '\t')) start++;
    try {
        return std::stoi(json.substr(start));
    } catch (...) {
        return defaultVal;
    }
}

static bool GetBoolProp(const std::string& json, const std::string& key, bool defaultVal) {
    std::string search = "\"" + key + "\":";
    size_t pos = json.find(search);
    if (pos == std::string::npos) return defaultVal;
    size_t start = pos + search.length();
    if (json.substr(start, 4) == "true") return true;
    if (json.substr(start, 5) == "false") return false;
    return defaultVal;
}

static void HandleWebMessage(const std::wstring& msgW) {
    if (msgW.empty()) return;
    try {
        LOG("[MSG] HandleWebMessage received: %ls", msgW.c_str());
        int len = WideCharToMultiByte(CP_UTF8, 0, msgW.c_str(), (int)msgW.length(), NULL, 0, NULL, NULL);
        if (len <= 0) return;
        std::string msg(len, '\0');
        WideCharToMultiByte(CP_UTF8, 0, msgW.c_str(), (int)msgW.length(), &msg[0], len, NULL, NULL);

        std::string cmd = GetStringProp(msg, "cmd");
        LOG("[MSG] Parsed cmd: %s", cmd.c_str());

        if (cmd == "get_subnets") {
            if (g_pfnGetSubnets) {
                char* pJson = g_pfnGetSubnets();
                if (pJson) {
                    int wlen = MultiByteToWideChar(CP_UTF8, 0, pJson, -1, NULL, 0);
                    if (wlen > 1) {
                        std::wstring wsubnets(wlen - 1, L'\0');
                        MultiByteToWideChar(CP_UTF8, 0, pJson, -1, &wsubnets[0], wlen);
                        std::wstring resp = L"{\"cmd\":\"subnets_result\",\"subnets\":" + wsubnets + L"}";
                        PostJsonToUI(resp);
                    }
                    if (g_pfnFreeCString) g_pfnFreeCString(pJson);
                }
            }
        } else if (cmd == "start_scan") {
            std::string cidr = GetStringProp(msg, "target_cidr");
            std::string ports = GetStringProp(msg, "ports");
            int concurrency = GetIntProp(msg, "concurrency", 256);
            bool scanPorts = GetBoolProp(msg, "scan_ports", true);

            LOG("[MSG] start_scan on %s with %d threads", cidr.c_str(), concurrency);
            if (g_pfnStartScan) {
                g_pfnStartScan(cidr.c_str(), ports.c_str(), concurrency, scanPorts ? 1 : 0, OnProgressFromGo);
            }
        } else if (cmd == "stop_scan") {
            LOG("[MSG] stop_scan requested");
            if (g_pfnStopScan) {
                g_pfnStopScan();
            }
        }
    } catch (const std::exception& ex) {
        LOG("[EXC] HandleWebMessage exception: %s", ex.what());
    } catch (...) {
        LOG("[EXC] HandleWebMessage unknown exception");
    }
}

static std::wstring LoadEmbeddedHTML() {
    HRSRC hRes = FindResourceW(NULL, MAKEINTRESOURCEW(IDR_INDEX_HTML), (LPCWSTR)RT_RCDATA);
    if (!hRes) return L"";
    HGLOBAL hGlobal = LoadResource(NULL, hRes);
    if (!hGlobal) return L"";
    DWORD size = SizeofResource(NULL, hRes);
    const char* pData = (const char*)LockResource(hGlobal);
    if (!pData || size == 0) return L"";

    int wlen = MultiByteToWideChar(CP_UTF8, 0, pData, size, NULL, 0);
    std::wstring html(wlen, 0);
    MultiByteToWideChar(CP_UTF8, 0, pData, size, &html[0], wlen);
    return html;
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
                g_pWebView->PostWebMessageAsJson(pJson);
            }
            free(pJson);
        }
        return 0;
    }

    case WM_DESTROY:
        if (g_pfnStopScan) g_pfnStopScan();
        if (g_pController) g_pController->Close();
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hWnd, msg, wParam, lParam);
}

int WINAPI WinMain(HINSTANCE hInstance, HINSTANCE, LPSTR, int nCmdShow) {
    LOG("[0] Program start");
    CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    OleInitialize(NULL);

    LOG("[2] Loading Go Engine...");
    bool engineOk = LoadGoEngine();
    LOG("[3] LoadGoEngine result: %d", engineOk);

    WNDCLASSEXW wc = { sizeof(wc) };
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
        LOG("[ERR] CreateWindowExW failed: %d", GetLastError());
        return 1;
    }
    LOG("[5] Window created: %p", (void*)g_hWnd);

    // Dark theme title bar
    BOOL dark = TRUE;
    DwmSetWindowAttribute(g_hWnd, 20 /* DWMWA_USE_IMMERSIVE_DARK_MODE */, &dark, sizeof(dark));

    ShowWindow(g_hWnd, SW_SHOWNORMAL);
    UpdateWindow(g_hWnd);
    SetForegroundWindow(g_hWnd);

    // Initialize WebView2 with process-unique user data folder to prevent 0x800700AA lock
    wchar_t tempPath[MAX_PATH];
    GetTempPathW(MAX_PATH, tempPath);
    std::wstring userDataFolder = std::wstring(tempPath) + L"QuickNetScan_WV_" + std::to_wstring(GetCurrentProcessId());
    CreateDirectoryW(userDataFolder.c_str(), NULL);
    LOG("[userDataFolder] %ls", userDataFolder.c_str());

    wchar_t exeDir[MAX_PATH];
    GetModuleFileNameW(NULL, exeDir, MAX_PATH);
    wchar_t* pSep = wcsrchr(exeDir, L'\\');
    if (pSep) *(pSep + 1) = L'\0';
    std::wstring loaderPath = std::wstring(exeDir) + L"WebView2Loader.dll";
    LOG("[6] Loading WebView2Loader: %ls", loaderPath.c_str());
    HMODULE hLoader = LoadLibraryW(loaderPath.c_str());
    if (!hLoader) hLoader = LoadLibraryW(L"WebView2Loader.dll");
    LOG("[7] hLoader: %p", (void*)hLoader);

    CreateEnvFn createEnv = hLoader ? (CreateEnvFn)GetProcAddress(hLoader, "CreateCoreWebView2EnvironmentWithOptions") : nullptr;
    LOG("[8] createEnv: %p", (void*)createEnv);

    auto onEnvCreated = [](HRESULT hr, ICoreWebView2Environment* env) -> HRESULT {
        LOG("[9] onEnvCreated: hr=0x%08X, env=%p", (unsigned)hr, (void*)env);
        if (FAILED(hr) || !env) return hr;
        env->CreateCoreWebView2Controller(g_hWnd, new ControllerCompletedHandler([](HRESULT hr2, ICoreWebView2Controller* controller) -> HRESULT {
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

            wchar_t exeDir[MAX_PATH];
            GetModuleFileNameW(NULL, exeDir, MAX_PATH);
            wchar_t* pSep = wcsrchr(exeDir, L'\\');
            if (pSep) *(pSep + 1) = L'\0';
            std::wstring webDir = std::wstring(exeDir) + L"web";
            LOG("[10-5] webDir: %ls", webDir.c_str());

            ICoreWebView2_3* webview3 = nullptr;
            HRESULT hrQI = g_pWebView->QueryInterface(IID_ICoreWebView2_3, (void**)&webview3);
            LOG("[10-6] QueryInterface ICoreWebView2_3: hr=0x%08X, ptr=%p", (unsigned)hrQI, (void*)webview3);

            if (SUCCEEDED(hrQI) && webview3) {
                HRESULT hrMap = webview3->SetVirtualHostNameToFolderMapping(
                    L"quicknetscan.local",
                    webDir.c_str(),
                    COREWEBVIEW2_HOST_RESOURCE_ACCESS_KIND_ALLOW
                );
                LOG("[10-7] SetVirtualHostNameToFolderMapping: hr=0x%08X", (unsigned)hrMap);
                webview3->Release();
                HRESULT hrNav = g_pWebView->Navigate(L"https://quicknetscan.local/index.html");
                LOG("[10-8] Navigate: hr=0x%08X", (unsigned)hrNav);
            } else {
                std::wstring fileUrl = L"file:///" + webDir + L"/index.html";
                LOG("[10-9] Navigate to fileUrl: %ls", fileUrl.c_str());
                g_pWebView->Navigate(fileUrl.c_str());
            }
            LOG("[10-10] Done setup");
            return S_OK;
        }));
        return S_OK;
    };

    if (createEnv) {
        LOG("[11] Calling createEnv...");
        HRESULT hrEnv = createEnv(nullptr, userDataFolder.c_str(), nullptr, new EnvCompletedHandler(onEnvCreated));
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
    return (int)msg.wParam;
}

#include "DebugLogConfig.h"

#include <Windows.h>

#include <algorithm>
#include <cstdio>
#include <string>

namespace Ime {

namespace {

enum class DebugLogLevel {
    Off,
    Debug,
    Trace,
};

std::wstring configFilePath() {
    const wchar_t* localAppData = _wgetenv(L"LOCALAPPDATA");
    if (!localAppData || !*localAppData) {
        return L"";
    }
    return std::wstring(localAppData) + L"\\MoqiIM\\MoqiLauncher.json";
}

bool readFileText(const std::wstring& path, std::string* out) {
    if (!out || path.empty()) {
        return false;
    }

    FILE* file = nullptr;
    if (_wfopen_s(&file, path.c_str(), L"rb") != 0 || !file) {
        return false;
    }

    std::string content;
    char buffer[512];
    size_t read = 0;
    while ((read = fread(buffer, 1, sizeof(buffer), file)) != 0) {
        content.append(buffer, read);
    }
    fclose(file);
    *out = std::move(content);
    return true;
}

DebugLogLevel parseDebugLogLevel(const std::string& raw) {
    std::string text = raw;
    std::transform(text.begin(), text.end(), text.begin(), [](unsigned char ch) {
        return static_cast<char>(std::tolower(ch));
    });

    const size_t keyPos = text.find("\"loglevel\"");
    if (keyPos == std::string::npos) {
        return DebugLogLevel::Off;
    }

    const size_t colonPos = text.find(':', keyPos);
    if (colonPos == std::string::npos) {
        return DebugLogLevel::Off;
    }

    const size_t firstQuote = text.find('"', colonPos);
    if (firstQuote == std::string::npos) {
        return DebugLogLevel::Off;
    }

    const size_t secondQuote = text.find('"', firstQuote + 1);
    if (secondQuote == std::string::npos) {
        return DebugLogLevel::Off;
    }

    const std::string value = text.substr(firstQuote + 1, secondQuote - firstQuote - 1);
    if (value == "trace") {
        return DebugLogLevel::Trace;
    }
    if (value == "debug") {
        return DebugLogLevel::Debug;
    }
    return DebugLogLevel::Off;
}

} // namespace

bool isDebugLoggingEnabled() {
    static ULONGLONG lastRefreshTick = 0;
    static DebugLogLevel cachedLevel = DebugLogLevel::Off;

    const ULONGLONG now = ::GetTickCount64();
    if (lastRefreshTick != 0 && now - lastRefreshTick < 1000) {
        return cachedLevel >= DebugLogLevel::Debug;
    }

    cachedLevel = DebugLogLevel::Off;
    std::string configText;
    if (readFileText(configFilePath(), &configText)) {
        cachedLevel = parseDebugLogLevel(configText);
    }

    lastRefreshTick = now;
    return cachedLevel >= DebugLogLevel::Debug;
}

bool isTraceLoggingEnabled() {
    static ULONGLONG lastRefreshTick = 0;
    static DebugLogLevel cachedLevel = DebugLogLevel::Off;

    const ULONGLONG now = ::GetTickCount64();
    if (lastRefreshTick != 0 && now - lastRefreshTick < 1000) {
        return cachedLevel >= DebugLogLevel::Trace;
    }

    cachedLevel = DebugLogLevel::Off;
    std::string configText;
    if (readFileText(configFilePath(), &configText)) {
        cachedLevel = parseDebugLogLevel(configText);
    }

    lastRefreshTick = now;
    return cachedLevel >= DebugLogLevel::Trace;
}

} // namespace Ime

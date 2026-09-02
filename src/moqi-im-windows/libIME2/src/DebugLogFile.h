#ifndef IME_DEBUG_LOG_FILE_H
#define IME_DEBUG_LOG_FILE_H
#pragma once

#include <Windows.h>

#include <cstdio>
#include <cwctype>
#include <ctime>
#include <mutex>
#include <string>
#include <unordered_map>

namespace Ime {
namespace DebugLogFile {

constexpr int kRetainDays = 7;

inline bool ensureDirectoryRecursive(const std::wstring& path) {
    if (path.empty()) {
        return false;
    }
    if (path.size() == 2 && path[1] == L':') {
        return true;
    }

    const DWORD attrs = ::GetFileAttributesW(path.c_str());
    if (attrs != INVALID_FILE_ATTRIBUTES) {
        return (attrs & FILE_ATTRIBUTE_DIRECTORY) != 0;
    }

    const size_t pos = path.find_last_of(L"\\/");
    if (pos != std::wstring::npos) {
        const std::wstring parent = path.substr(0, pos);
        if (!parent.empty() && parent != path && !ensureDirectoryRecursive(parent)) {
            return false;
        }
    }

    return ::CreateDirectoryW(path.c_str(), nullptr) != FALSE ||
           ::GetLastError() == ERROR_ALREADY_EXISTS;
}

inline std::wstring joinPath(const std::wstring& dir, const std::wstring& fileName) {
    if (dir.empty()) {
        return fileName;
    }
    if (dir.back() == L'\\' || dir.back() == L'/') {
        return dir + fileName;
    }
    return dir + L"\\" + fileName;
}

inline std::wstring currentDateStamp() {
    SYSTEMTIME st{};
    ::GetLocalTime(&st);
    wchar_t buffer[16] = {};
    _snwprintf_s(buffer, _countof(buffer), _TRUNCATE, L"%04u-%02u-%02u",
                 st.wYear, st.wMonth, st.wDay);
    return buffer;
}

inline std::wstring cutoffDateStamp(int retainDays = kRetainDays) {
    std::time_t now = std::time(nullptr);
    if (retainDays > 1) {
        now -= static_cast<std::time_t>(retainDays - 1) * 24 * 60 * 60;
    }

    std::tm localTm{};
    localtime_s(&localTm, &now);

    wchar_t buffer[16] = {};
    _snwprintf_s(buffer, _countof(buffer), _TRUNCATE, L"%04d-%02d-%02d",
                 localTm.tm_year + 1900, localTm.tm_mon + 1, localTm.tm_mday);
    return buffer;
}

inline bool isDateStamp(const std::wstring& text) {
    return text.size() == 10 &&
           std::iswdigit(text[0]) && std::iswdigit(text[1]) &&
           std::iswdigit(text[2]) && std::iswdigit(text[3]) &&
           text[4] == L'-' &&
           std::iswdigit(text[5]) && std::iswdigit(text[6]) &&
           text[7] == L'-' &&
           std::iswdigit(text[8]) && std::iswdigit(text[9]);
}

inline std::wstring dailyFileName(const std::wstring& fileName,
                                  const std::wstring& dateStamp = currentDateStamp()) {
    const size_t dotPos = fileName.rfind(L'.');
    if (dotPos == std::wstring::npos) {
        return fileName + L"-" + dateStamp;
    }
    return fileName.substr(0, dotPos) + L"-" + dateStamp + fileName.substr(dotPos);
}

inline void cleanupOldDailyLogs(const std::wstring& logDir, const std::wstring& fileName,
                                int retainDays = kRetainDays) {
    const size_t dotPos = fileName.rfind(L'.');
    const std::wstring prefix =
        dotPos == std::wstring::npos ? fileName + L"-" : fileName.substr(0, dotPos) + L"-";
    const std::wstring suffix =
        dotPos == std::wstring::npos ? L"" : fileName.substr(dotPos);
    const std::wstring cutoff = cutoffDateStamp(retainDays);
    const std::wstring searchPattern = joinPath(logDir, prefix + L"*");

    WIN32_FIND_DATAW findData{};
    HANDLE findHandle = ::FindFirstFileW(searchPattern.c_str(), &findData);
    if (findHandle == INVALID_HANDLE_VALUE) {
        return;
    }

    do {
        if ((findData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) != 0) {
            continue;
        }

        const std::wstring candidate = findData.cFileName;
        if (candidate.compare(0, prefix.size(), prefix) != 0 ||
            candidate.size() < prefix.size() + 10 + suffix.size()) {
            continue;
        }

        const std::wstring dateStamp = candidate.substr(prefix.size(), 10);
        if (!isDateStamp(dateStamp)) {
            continue;
        }

        const size_t suffixPos = prefix.size() + 10;
        if (!suffix.empty() &&
            candidate.compare(suffixPos, suffix.size(), suffix) != 0) {
            continue;
        }

        if (dateStamp < cutoff) {
            ::DeleteFileW(joinPath(logDir, candidate).c_str());
        }
    } while (::FindNextFileW(findHandle, &findData) != FALSE);

    ::FindClose(findHandle);
}

inline bool shouldRunDailyCleanup(const std::wstring& logDir, const std::wstring& fileName,
                                  const std::wstring& dateStamp) {
    static std::mutex mutex;
    static std::unordered_map<std::wstring, std::wstring> lastCleanupDate;

    const std::wstring key = logDir + L"|" + fileName;
    std::lock_guard<std::mutex> lock(mutex);
    auto it = lastCleanupDate.find(key);
    if (it != lastCleanupDate.end() && it->second == dateStamp) {
        return false;
    }
    lastCleanupDate[key] = dateStamp;
    return true;
}

inline std::wstring prepareDailyLogFilePath(const std::wstring& logDir,
                                            const std::wstring& fileName) {
    if (!ensureDirectoryRecursive(logDir)) {
        return L"";
    }

    const std::wstring dateStamp = currentDateStamp();
    if (shouldRunDailyCleanup(logDir, fileName, dateStamp)) {
        cleanupOldDailyLogs(logDir, fileName);
    }

    return joinPath(logDir, dailyFileName(fileName, dateStamp));
}

} // namespace DebugLogFile
} // namespace Ime

#endif

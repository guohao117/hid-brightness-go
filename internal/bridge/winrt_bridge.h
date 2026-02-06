#pragma once
#include <stdint.h>

#ifdef _WIN32
#define DLL_EXPORT __declspec(dllexport)
#else
#define DLL_EXPORT
#endif

#ifdef __cplusplus
extern "C" {
#endif

// Generic init/uninit
DLL_EXPORT int winrt_initialize();
DLL_EXPORT void winrt_uninitialize();

// ALS Functions
typedef void (*ALSReadingChangedCallback)(void* context, uint64_t luxBits);

DLL_EXPORT int winrt_als_initialize();
DLL_EXPORT void* winrt_als_open_default();
DLL_EXPORT void winrt_als_set_report_threshold(void* handle, double thresholdLux);
DLL_EXPORT int winrt_als_get_lux(void* handle, double* outLux);
DLL_EXPORT int64_t winrt_als_subscribe_reading_changed(void* handle, ALSReadingChangedCallback callback, void* context);
DLL_EXPORT void winrt_als_unsubscribe_reading_changed(void* handle, int64_t token);
DLL_EXPORT void* winrt_als_enumerate();
DLL_EXPORT int winrt_als_get_device_count(void* handle);
DLL_EXPORT void winrt_als_get_device_properties(void* handle, int index, char* outId, int maxIdLen, char* outName, int maxNameLen, uint16_t* outVid, uint16_t* outPid);
DLL_EXPORT void winrt_als_free_enumeration(void* handle);
DLL_EXPORT void* winrt_als_open_by_id(const char* deviceId);
DLL_EXPORT void winrt_als_close(void* handle);
DLL_EXPORT void winrt_als_uninitialize();

// HID Functions
DLL_EXPORT int winrt_hid_initialize();
DLL_EXPORT void* winrt_hid_enumerate(uint16_t usagePage, uint16_t usageId);
DLL_EXPORT int winrt_hid_get_device_count(void* handle);
DLL_EXPORT void winrt_hid_get_device_info(void* handle, int index, char* outId, int maxIdLen, char* outName, int maxNameLen, uint16_t* outVid, uint16_t* outPid, uint16_t* outUsagePage, uint16_t* outUsageId);
DLL_EXPORT void winrt_hid_free_enumeration(void* handle);
DLL_EXPORT void* winrt_hid_open(const char* deviceId);
DLL_EXPORT int winrt_hid_get_feature_report(void* handle, uint16_t reportId, uint8_t* buffer, uint32_t bufferLen, uint32_t* bytesWritten);
DLL_EXPORT int winrt_hid_send_feature_report(void* handle, uint16_t reportId, const uint8_t* buffer, uint32_t bufferLen);
DLL_EXPORT void winrt_hid_close(void* handle);
DLL_EXPORT void winrt_hid_uninitialize();

#ifdef __cplusplus
}
#endif

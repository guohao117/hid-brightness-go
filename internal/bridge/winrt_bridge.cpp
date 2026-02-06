#include <unknwn.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Devices.Sensors.h>
#include <winrt/Windows.Devices.HumanInterfaceDevice.h>
#include <winrt/Windows.Devices.Enumeration.h>
#include <winrt/Windows.Storage.Streams.h>
#include <vector>
#include <string>
#include <cstring>
#include <cstdio>
#include "winrt_bridge.h"

using namespace winrt;
using namespace Windows::Devices::Sensors;
using namespace Windows::Devices::HumanInterfaceDevice;
using namespace Windows::Devices::Enumeration;
using namespace Windows::Storage::Streams;

// --- Internal Wrappers ---

struct SensorWrapper {
    LightSensor sensor{ nullptr };
};

struct HidDeviceWrapper {
    HidDevice device{ nullptr };
};

struct DeviceListWrapper {
    std::vector<DeviceInformation> devices;
};

extern "C" {

// --- Lifecycle ---

DLL_EXPORT int winrt_initialize() {
    try {
        init_apartment();
        return 0;
    } catch (...) {
        return -1;
    }
}

DLL_EXPORT void winrt_uninitialize() {
    try {
        uninit_apartment();
    } catch (...) {}
}

DLL_EXPORT int winrt_als_initialize() { return winrt_initialize(); }
DLL_EXPORT void winrt_als_uninitialize() { winrt_uninitialize(); }
DLL_EXPORT int winrt_hid_initialize() { return winrt_initialize(); }
DLL_EXPORT void winrt_hid_uninitialize() { winrt_uninitialize(); }

// --- ALS Implementation ---

DLL_EXPORT void* winrt_als_open_default() {
    try {
        LightSensor sensor = LightSensor::GetDefault();
        if (!sensor) return nullptr;
        return new SensorWrapper{ sensor };
    } catch (...) {
        return nullptr;
    }
}

DLL_EXPORT void winrt_als_set_report_threshold(void* handle, double thresholdLux) {
    if (!handle) return;
    try {
        SensorWrapper* wrapper = static_cast<SensorWrapper*>(handle);
        if (wrapper && wrapper->sensor) {
            auto threshold = wrapper->sensor.ReportThreshold();
            if (threshold) {
                threshold.AbsoluteLux(static_cast<float>(thresholdLux));
            }
        }
    } catch (...) {}
}

DLL_EXPORT int winrt_als_get_lux(void* handle, double* outLux) {
    if (!handle || !outLux) return -1;
    try {
        SensorWrapper* wrapper = static_cast<SensorWrapper*>(handle);
        if (!wrapper || !wrapper->sensor) return -1;
        LightSensorReading reading = wrapper->sensor.GetCurrentReading();
        if (!reading) return -1;
        *outLux = reading.IlluminanceInLux();
        return 0;
    } catch (...) { return -1; }
}

DLL_EXPORT int64_t winrt_als_subscribe_reading_changed(void* handle, ALSReadingChangedCallback callback, void* context) {
    if (!handle || !callback) return 0;
    try {
        SensorWrapper* wrapper = static_cast<SensorWrapper*>(handle);
        if (!wrapper || !wrapper->sensor) return 0;
        
        event_token token = wrapper->sensor.ReadingChanged([callback, context](LightSensor const&, LightSensorReadingChangedEventArgs const& args) {
            if (callback) {
                double lux = args.Reading().IlluminanceInLux();
                uint64_t bits;
                std::memcpy(&bits, &lux, sizeof(uint64_t));
                callback(context, bits);
            }
        });
        return token.value;
    } catch (...) { return 0; }
}

DLL_EXPORT void winrt_als_unsubscribe_reading_changed(void* handle, int64_t tokenValue) {
    if (!handle) return;
    try {
        SensorWrapper* wrapper = static_cast<SensorWrapper*>(handle);
        if (wrapper && wrapper->sensor) {
            event_token token;
            token.value = tokenValue;
            wrapper->sensor.ReadingChanged(token);
        }
    } catch (...) {}
}

DLL_EXPORT void* winrt_als_enumerate() {
    try {
        hstring selector = LightSensor::GetDeviceSelector();
        auto requestedProperties = winrt::single_threaded_vector<hstring>({
            L"System.DeviceInterface.Hid.VendorId",
            L"System.DeviceInterface.Hid.ProductId"
        });
        DeviceInformationCollection collection = DeviceInformation::FindAllAsync(selector, requestedProperties).get();
        DeviceListWrapper* wrapper = new DeviceListWrapper();
        for (auto const& device : collection) wrapper->devices.push_back(device);
        return wrapper;
    } catch (...) { return nullptr; }
}

DLL_EXPORT int winrt_als_get_device_count(void* handle) {
    if (!handle) return 0;
    return static_cast<int>(static_cast<DeviceListWrapper*>(handle)->devices.size());
}

DLL_EXPORT void winrt_als_get_device_properties(void* handle, int index, char* outId, int maxIdLen, char* outName, int maxNameLen, uint16_t* outVid, uint16_t* outPid) {
    if (!handle) return;
    DeviceListWrapper* wrapper = static_cast<DeviceListWrapper*>(handle);
    if (index < 0 || index >= static_cast<int>(wrapper->devices.size())) return;
    auto const& device = wrapper->devices[index];
    std::string id = to_string(device.Id());
    std::string name = to_string(device.Name());
    if (outId && maxIdLen > 0) strncpy_s(outId, maxIdLen, id.c_str(), _TRUNCATE);
    if (outName && maxNameLen > 0) strncpy_s(outName, maxNameLen, name.c_str(), _TRUNCATE);
    if (outVid) {
        auto prop = device.Properties().TryLookup(L"System.DeviceInterface.Hid.VendorId");
        *outVid = prop ? unbox_value<uint16_t>(prop) : 0;
    }
    if (outPid) {
        auto prop = device.Properties().TryLookup(L"System.DeviceInterface.Hid.ProductId");
        *outPid = prop ? unbox_value<uint16_t>(prop) : 0;
    }
}

DLL_EXPORT void winrt_als_free_enumeration(void* handle) {
    if (handle) delete static_cast<DeviceListWrapper*>(handle);
}

DLL_EXPORT void* winrt_als_open_by_id(const char* deviceId) {
    if (!deviceId) return nullptr;
    try {
        hstring id = to_hstring(deviceId);
        LightSensor sensor = LightSensor::FromIdAsync(id).get();
        if (!sensor) return nullptr;
        return new SensorWrapper{ sensor };
    } catch (...) { return nullptr; }
}

DLL_EXPORT void winrt_als_close(void* handle) {
    if (handle) delete static_cast<SensorWrapper*>(handle);
}

// --- HID Implementation ---

DLL_EXPORT void* winrt_hid_enumerate(uint16_t usagePage, uint16_t usageId) {
    try {
        hstring selector;
        if (usagePage == 0) {
            selector = L"System.Devices.InterfaceClassGuid:=\"{4D1E55B2-F16F-11CF-88CB-001111000030}\" AND System.Devices.InterfaceEnabled:=System.StructuredQueryType.Boolean#True";
        } else {
            selector = HidDevice::GetDeviceSelector(usagePage, usageId);
        }
        auto requestedProperties = winrt::single_threaded_vector<hstring>({
            L"System.DeviceInterface.Hid.VendorId",
            L"System.DeviceInterface.Hid.ProductId",
            L"System.DeviceInterface.Hid.UsagePage",
            L"System.DeviceInterface.Hid.UsageId"
        });
        DeviceInformationCollection collection = DeviceInformation::FindAllAsync(selector, requestedProperties).get();
        DeviceListWrapper* wrapper = new DeviceListWrapper();
        for (auto const& device : collection) wrapper->devices.push_back(device);
        return wrapper;
    } catch (...) { return nullptr; }
}

DLL_EXPORT int winrt_hid_get_device_count(void* handle) {
    if (!handle) return 0;
    return static_cast<int>(static_cast<DeviceListWrapper*>(handle)->devices.size());
}

DLL_EXPORT void winrt_hid_get_device_info(void* handle, int index, char* outId, int maxIdLen, char* outName, int maxNameLen, uint16_t* outVid, uint16_t* outPid, uint16_t* outUsagePage, uint16_t* outUsageId) {
    if (!handle) return;
    DeviceListWrapper* wrapper = static_cast<DeviceListWrapper*>(handle);
    if (index < 0 || index >= static_cast<int>(wrapper->devices.size())) return;
    auto const& device = wrapper->devices[index];
    std::string id = to_string(device.Id());
    std::string name = to_string(device.Name());
    if (outId && maxIdLen > 0) strncpy_s(outId, maxIdLen, id.c_str(), _TRUNCATE);
    if (outName && maxNameLen > 0) strncpy_s(outName, maxNameLen, name.c_str(), _TRUNCATE);

    auto getProp = [&](const wchar_t* key, uint16_t* out) {
        if (!out) return;
        auto prop = device.Properties().TryLookup(key);
        if (prop) {
            try { *out = unbox_value<uint16_t>(prop); } catch (...) { *out = 0; }
        } else { *out = 0; }
    };
    getProp(L"System.DeviceInterface.Hid.VendorId", outVid);
    getProp(L"System.DeviceInterface.Hid.ProductId", outPid);
    getProp(L"System.DeviceInterface.Hid.UsagePage", outUsagePage);
    getProp(L"System.DeviceInterface.Hid.UsageId", outUsageId);
}

DLL_EXPORT void winrt_hid_free_enumeration(void* handle) {
    if (handle) delete static_cast<DeviceListWrapper*>(handle);
}

DLL_EXPORT void* winrt_hid_open(const char* deviceId) {
    if (!deviceId) return nullptr;
    try {
        hstring id = to_hstring(deviceId);
        HidDevice device = HidDevice::FromIdAsync(id, Windows::Storage::FileAccessMode::ReadWrite).get();
        if (!device) return nullptr;
        return new HidDeviceWrapper{ device };
    } catch (...) { return nullptr; }
}

DLL_EXPORT int winrt_hid_get_feature_report(void* handle, uint16_t reportId, uint8_t* buffer, uint32_t bufferLen, uint32_t* bytesWritten) {
    if (!handle || !buffer || !bytesWritten) return -1;
    try {
        HidDeviceWrapper* wrapper = static_cast<HidDeviceWrapper*>(handle);
        HidFeatureReport report = wrapper->device.GetFeatureReportAsync(reportId).get();
        if (!report) return -1;
        IBuffer data = report.Data();
        uint32_t dataLen = data.Length();
        *bytesWritten = (dataLen < bufferLen) ? dataLen : bufferLen;
        auto reader = DataReader::FromBuffer(data);
        reader.ReadBytes(winrt::array_view<uint8_t>(buffer, buffer + *bytesWritten));
        return 0;
    } catch (...) { return -1; }
}

DLL_EXPORT int winrt_hid_send_feature_report(void* handle, uint16_t reportId, const uint8_t* buffer, uint32_t bufferLen) {
    if (!handle || !buffer) return -1;
    try {
        HidDeviceWrapper* wrapper = static_cast<HidDeviceWrapper*>(handle);
        HidFeatureReport report = wrapper->device.CreateFeatureReport(reportId);
        DataWriter writer;
        writer.WriteBytes(winrt::array_view<const uint8_t>(buffer, buffer + bufferLen));
        report.Data(writer.DetachBuffer());
        wrapper->device.SendFeatureReportAsync(report).get();
        return 0;
    } catch (const winrt::hresult_error& ex) { return (int)ex.code().value; }
    catch (...) { return -1; }
}

DLL_EXPORT void winrt_hid_close(void* handle) {
    if (handle) delete static_cast<HidDeviceWrapper*>(handle);
}

}

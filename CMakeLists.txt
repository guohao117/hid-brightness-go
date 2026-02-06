cmake_minimum_required(VERSION 3.15)
project(HIDBrightnessGo LANGUAGES CXX)

find_program(RSRC_EXECUTABLE rsrc)

# 设置 C++ 标准
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 统一设置输出目录为项目根目录下的 bin
# 注意：对于多配置生成器（如 Visual Studio），CMake 会在后面自动加上 /Release 或 /Debug
set(OUTPUT_DIR ${CMAKE_SOURCE_DIR}/bin)
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${OUTPUT_DIR})
set(CMAKE_LIBRARY_OUTPUT_DIRECTORY ${OUTPUT_DIR})
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${OUTPUT_DIR})

# 1. 定义 DLL 项目
add_library(winrt_bridge SHARED 
    internal/bridge/winrt_bridge.cpp
    internal/bridge/winrt_bridge.h
)

# 链接 Windows 系统库
target_link_libraries(winrt_bridge PRIVATE windowsapp.lib)

if(MSVC)
    target_compile_options(winrt_bridge PRIVATE /EHsc /W4)
endif()

# 2. 定义 Go 编译任务
set(GO_MANIFEST_FILE "${CMAKE_SOURCE_DIR}/cmd/hid-brightness/ultrafine-brightness.exe.manifest")
set(GO_ICON_FILE "${CMAKE_SOURCE_DIR}/cmd/hid-brightness/app.ico")
set(GO_RESOURCE_FILE "${CMAKE_SOURCE_DIR}/cmd/hid-brightness/rsrc.syso")

if(RSRC_EXECUTABLE)
    # Use relative paths for rsrc commands to avoid issues, or absolute paths if rsrc supports them.
    # The output is defined as absolute path in GO_RESOURCE_FILE.
    
    add_custom_command(
        OUTPUT "${GO_RESOURCE_FILE}"
        COMMAND "${RSRC_EXECUTABLE}" -manifest "${GO_MANIFEST_FILE}" -ico "${GO_ICON_FILE}" -o "${GO_RESOURCE_FILE}"
        DEPENDS "${GO_MANIFEST_FILE}" "${GO_ICON_FILE}"
        WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
        COMMENT "Generating Windows resource file (rsrc.syso)..."
        VERBATIM
    )

    add_custom_target(go_resources DEPENDS "${GO_RESOURCE_FILE}")
    list(APPEND GO_APP_DEPENDS go_resources)
else()
    message(WARNING "rsrc tool not found. The build will proceed, but if 'cmd/hid-brightness/rsrc.syso' is missing or outdated, the icon/manifest won't be updated.")
    # Assuming rsrc.syso exists or go build will just skip it (resulting in default icon)
endif()

add_custom_target(go_app ALL
    COMMAND go build -ldflags "-H=windowsgui" -o "$<TARGET_FILE_DIR:winrt_bridge>/ultrafine-brightness.exe" "./cmd/hid-brightness"
    WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
    DEPENDS winrt_bridge ${GO_APP_DEPENDS}
    COMMENT "Building Go executable next to DLL..."
    VERBATIM
)

# 3. 定义测试工具编译任务
add_custom_target(test_hid ALL
    COMMAND go build -v -o "$<TARGET_FILE_DIR:winrt_bridge>/test-hid.exe" "./cmd/test-hid"
    WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
    DEPENDS winrt_bridge
    COMMENT "Building HID test tool..."
    VERBATIM
)

cmake_minimum_required(VERSION 3.22.1)

project(ptranslate_whisper C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# whisper.cpp is a git submodule at the repository root.
set(WHISPER_DIR ${CMAKE_CURRENT_SOURCE_DIR}/../../../../../third_party/whisper.cpp)

# Build whisper.cpp and ggml as static libraries that are linked into our one
# shared library, so the APK carries a single .so.
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_SERVER OFF CACHE BOOL "" FORCE)
# Without OpenMP ggml uses its own thread pool, and there is no libomp.so to
# package.
set(GGML_OPENMP OFF CACHE BOOL "" FORCE)
# "Native" means tuned for the build machine, which is not the phone.
set(GGML_NATIVE OFF CACHE BOOL "" FORCE)
set(GGML_CCACHE OFF CACHE BOOL "" FORCE)

add_subdirectory(${WHISPER_DIR} whisper.cpp)

# Debug builds of the app would otherwise compile the inference code with no
# optimisation, which makes it many times slower. The model code is always
# optimised; only our small bridge follows the app's build type.
foreach(target whisper ggml ggml-base ggml-cpu)
    if(TARGET ${target})
        target_compile_options(${target} PRIVATE -O3)
        target_compile_definitions(${target} PRIVATE NDEBUG)
    endif()
endforeach()

add_library(ptranslate_whisper SHARED whisper_jni.cpp)
target_link_libraries(ptranslate_whisper PRIVATE whisper log)

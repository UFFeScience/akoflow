cmake_minimum_required(VERSION 3.16)
project(akoflow_simgrid_runner LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(PkgConfig REQUIRED)
pkg_check_modules(SimGrid REQUIRED IMPORTED_TARGET simgrid)
find_package(nlohmann_json REQUIRED)

add_executable(akoflow-simgrid-runner src/main.cpp)
target_link_libraries(akoflow-simgrid-runner PRIVATE PkgConfig::SimGrid nlohmann_json::nlohmann_json)
target_compile_options(akoflow-simgrid-runner PRIVATE -Wall -Wextra -Wpedantic)

install(TARGETS akoflow-simgrid-runner RUNTIME DESTINATION bin)

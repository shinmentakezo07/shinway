cmake_minimum_required(VERSION 3.16)
project(shinway_management_api_c C)

add_library(shinway_management_api_c SHARED src/plugin.c)
set_target_properties(shinway_management_api_c PROPERTIES
    OUTPUT_NAME "management-api-c"
    PREFIX ""
)

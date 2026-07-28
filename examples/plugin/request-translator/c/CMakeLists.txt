cmake_minimum_required(VERSION 3.16)
project(shinway_request_translator_c C)

add_library(shinway_request_translator_c SHARED src/plugin.c)
set_target_properties(shinway_request_translator_c PROPERTIES
    OUTPUT_NAME "request-translator-c"
    PREFIX ""
)

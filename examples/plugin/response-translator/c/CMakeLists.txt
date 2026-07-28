cmake_minimum_required(VERSION 3.16)
project(shinway_response_translator_c C)

add_library(shinway_response_translator_c SHARED src/plugin.c)
set_target_properties(shinway_response_translator_c PROPERTIES
    OUTPUT_NAME "response-translator-c"
    PREFIX ""
)

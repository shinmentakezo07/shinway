cmake_minimum_required(VERSION 3.16)
project(shinway_response_normalizer_c C)

add_library(shinway_response_normalizer_c SHARED src/plugin.c)
set_target_properties(shinway_response_normalizer_c PROPERTIES
    OUTPUT_NAME "response-normalizer-c"
    PREFIX ""
)

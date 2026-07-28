cmake_minimum_required(VERSION 3.16)
project(shinway_protocol_format_c C)

add_library(shinway_protocol_format_c SHARED src/plugin.c)
set_target_properties(shinway_protocol_format_c PROPERTIES
    OUTPUT_NAME "protocol-format-c"
    PREFIX ""
)

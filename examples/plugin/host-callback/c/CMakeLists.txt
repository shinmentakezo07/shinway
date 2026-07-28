cmake_minimum_required(VERSION 3.16)
project(shinway_host_callback_c C)

add_library(shinway_host_callback_c SHARED src/plugin.c)
set_target_properties(shinway_host_callback_c PROPERTIES
    OUTPUT_NAME "host-callback-c"
    PREFIX ""
)

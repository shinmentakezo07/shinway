cmake_minimum_required(VERSION 3.16)
project(shinway_cli_c C)

add_library(shinway_cli_c SHARED src/plugin.c)
set_target_properties(shinway_cli_c PROPERTIES
    OUTPUT_NAME "cli-c"
    PREFIX ""
)

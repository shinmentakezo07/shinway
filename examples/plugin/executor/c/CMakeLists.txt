cmake_minimum_required(VERSION 3.16)
project(shinway_executor_c C)

add_library(shinway_executor_c SHARED src/plugin.c)
set_target_properties(shinway_executor_c PROPERTIES
    OUTPUT_NAME "executor-c"
    PREFIX ""
)

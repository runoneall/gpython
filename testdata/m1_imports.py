# M1 三级导入测试：标准库与本地导入
import math
import os
import sys

assert math.floor(3.7) == 3
assert math.pi > 3.14
assert os.sep == "/"
assert "sys" in sys.modules
assert "os" in sys.modules

# 本地导入：同目录下的 helper 模块
import m1_helper

assert m1_helper.double(5) == 10
assert m1_helper.PIE == 3
assert m1_helper.greet("gpython") == "hello, gpython"

print("m1_imports: all checks passed")

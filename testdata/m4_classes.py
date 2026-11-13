# M4 类、继承、异常与 try/except 冒烟测试
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def dist(self):
        return (self.x * self.x + self.y * self.y) ** 0.5


p = Point(3, 4)
print(p.dist())

try:
    1 / 0
except ZeroDivisionError:
    print("caught div by zero")

try:
    undefined_name
except NameError:
    print("caught name error")


class Animal:
    def speak(self):
        return "animal sound"


class Dog(Animal):
    def speak(self):
        return "woof"


d = Dog()
print(d.speak())

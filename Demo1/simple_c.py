import pygame
import sys
import socket
import json
import threading

pygame.init()
WIDTH, HEIGHT = 1280, 800
screen = pygame.display.set_mode((WIDTH, HEIGHT))
pygame.display.set_caption("100小球压力测试")

# 全局存储所有玩家
all_players = {}
self_addr = ""

# 简化小球类（仅保留核心坐标）


class Ball:
    def __init__(self, x, y, r, color, speed):
        self.x = x
        self.y = y
        self.r = r
        self.color = color
        self.speed = speed


# 初始化自己的小球
ball = Ball(WIDTH // 2, HEIGHT // 2, 20, (255, 0, 0), 5)

# 连接服务端
client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    client_socket.connect(("127.0.0.1", 16543))
    self_addr = client_socket.getsockname()
except:
    sys.exit()

# 接收服务端数据（无打印）


def receive_server_data():
    global all_players
    buffer = ""
    while True:
        try:
            data = client_socket.recv(8192).decode("utf-8")
            if not data:
                break
            buffer += data
            while "\n" in buffer:
                line, buffer = buffer.split("\n", 1)
                line = line.strip()
                if line:
                    all_players = json.loads(line)
        except:
            break


threading.Thread(target=receive_server_data, daemon=True).start()

# 主循环（高性能渲染）
clock = pygame.time.Clock()
while True:
    for event in pygame.event.get():
        if event.type == pygame.QUIT:
            pygame.quit()
            sys.exit()

    # 移动控制
    keys = pygame.key.get_pressed()
    speed = ball.speed * \
        1.5 if (keys[pygame.K_LSHIFT] or keys[pygame.K_RSHIFT]) else ball.speed
    if keys[pygame.K_LEFT]:
        ball.x = max(ball.x - speed, 20)
    if keys[pygame.K_RIGHT]:
        ball.x = min(ball.x + speed, WIDTH-20)
    if keys[pygame.K_UP]:
        ball.y = max(ball.y - speed, 20)
    if keys[pygame.K_DOWN]:
        ball.y = min(ball.y + speed, HEIGHT-20)

    # 绘制
    screen.fill((0, 0, 0))
    # 绘制1000个其他小球（蓝色）
    for addr, pos in all_players.items():
        pygame.draw.circle(screen, (0, 0, 255),
                           (int(pos["X"]), int(pos["Y"])), 15)
    # 绘制自己（红色置顶）
    pygame.draw.circle(screen, ball.color, (int(ball.x), int(ball.y)), 20)

    # 发送坐标
    client_socket.send((json.dumps({"X": ball.x, "Y": ball.y})+"\n").encode())

    pygame.display.flip()
    clock.tick(60)

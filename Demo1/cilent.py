# Pygame客户端

import pygame
import sys
import socket
import json
import threading
from collections import deque
import struct

pygame.init()

WIDTH, HEIGHT = 1280, 800

HEADER_SIZE = 8  # 4字节类型+4字节长度


MSG_TYPE_POS = 1  # 位置同步消息
PLAYER_SIZE = 20  # 每个玩家20字节


screen = pygame.display.set_mode((WIDTH, HEIGHT))
pygame.display.set_caption("Demo1-Cilent")


# 小球参数
class Ball:
    def __init__(self, x, y, r, color, speed):
        self.x = x
        self.y = y
        self.r = r
        self.color = color
        self.init_speed = speed
        self.cur_speed = speed
        self.trail = Trail(self, color)
        self.check_boundary()

    def check_boundary(self):
        # 边界校验
        if self.init_speed < 1:
            self.init_speed = 1
        if self.cur_speed < 1:
            self.cur_speed = 1
        if self.x < self.r:
            self.x = self.r
        if self.x + self.r > WIDTH:
            self.x = WIDTH - self.r
        if self.y < self.r:
            self.y = self.r
        if self.y + self.r > HEIGHT:
            self.y = HEIGHT - self.r


# 轨迹类
class Trail:
    """
    轨迹，用于记录小球移动路线
    :param ball: 小球对象
    :param color: 轨迹颜色（基础色，渐变将基于此色进行变化）
    :param max_points: 最大记录点数，默认-1表示不限制
    :param record_frames: 每隔record_frames帧记录一次点，默认3
    """

    def __init__(self, ball, color, max_points=100, record_frames=2):
        self.ball = ball
        self.color = color  # 基础颜色，用于计算渐变
        self.max_points = max_points
        self.record_frames = record_frames
        self.points = deque(maxlen=max_points if max_points > 0 else None)
        self.frame_count = 0

    def _record(self):
        self.points.append((self.ball.x, self.ball.y))

    def record_per_frame(self):
        """请在每一帧调用此函数。
        程序会自动计算间隔"""
        frame_count = self.frame_count
        if frame_count % self.record_frames == 0:
            self._record()
        self.frame_count += 1

    def clear(self):
        """清空记录"""
        self.points.clear()
        self.frame_count = 0

    def draw_gradient(self, screen):
        """绘制渐变轨迹线
        根据点在轨迹中的位置实现彩虹渐变效果
        """
        if len(self.points) < 2:
            return

        points_list = list(self.points)
        total = len(points_list)

        for i in range(total - 1):
            # 计算当前段在轨迹中的位置比例 (0-1)
            ratio = i / (total - 1) if total > 1 else 0

            # 彩虹渐变：根据比例计算色相 (0-360)
            hue = ratio * 360
            hue_prime = hue / 60
            x = int(255 * (1 - abs(hue_prime % 2 - 1)))

            # HSV 转 RGB
            if hue_prime < 1:
                r, g, b = 255, x, 0
            elif hue_prime < 2:
                r, g, b = x, 255, 0
            elif hue_prime < 3:
                r, g, b = 0, 255, x
            elif hue_prime < 4:
                r, g, b = 0, x, 255
            elif hue_prime < 5:
                r, g, b = x, 0, 255
            else:
                r, g, b = 255, 0, x

            # 绘制线段
            pygame.draw.line(screen, (r, g, b),
                             points_list[i], points_list[i+1], 3)


# 所有玩家(ID:(X,Y))
all_players = {}
# 时钟
clock = pygame.time.Clock()
# 自己小球
ball = Ball(WIDTH // 2, HEIGHT // 2, 20, (255, 0, 0), 5)


# 网络连接相关变量
cilent_socket = None
connected = False

try:
    cilent_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    cilent_socket.connect(("127.0.0.1", 16543))
    connected = True
    print("Connected to server. Multiplayer mode enabled.")
except Exception as e:
    print("Connect server error:", e)
    print("Running in single-player mode.")
    connected = False


def receive_server_data():
    recv_buffer = bytearray()
    while True:
        if not connected:
            break
        try:
            data = cilent_socket.recv(4096)
            if not data:
                continue
            recv_buffer.extend(data)
            # print(data)
            # 这里要解析二进制数据了，发送时格式：
            # 遍历positions，按小端序写
            # for _, pos := range positions {
            #     _ = binary.Write(buf, binary.LittleEndian, pos.ID)
            #     _ = binary.Write(buf, binary.LittleEndian, pos.X)
            #     _ = binary.Write(buf, binary.LittleEndian, pos.Y)
            # }
            # binData := buf.Bytes()
            #
            # 每个玩家数据固定20字节，前4字节为ID，后16字节为X,Y坐标
            #

            # 读8字节包头
            while len(recv_buffer) >= HEADER_SIZE:
                # 取8字节
                header = recv_buffer[:HEADER_SIZE]
                msg_type, body_len = struct.unpack("<II", header)

                # 总数据长度
                full_packet_len = HEADER_SIZE + body_len

                # 等待缓存区包
                while len(recv_buffer) < full_packet_len:
                    break

                # 取包
                full_packet = recv_buffer[:full_packet_len]
                del recv_buffer[:full_packet_len]

                # 提取消息体
                body_data = full_packet[HEADER_SIZE:]

                # 分配给处理函数
                handle_server_message(msg_type, body_data)

        except Exception as e:
            print("Receive server error:", e)
            break


def handle_server_message(msg_type, body_data):
    global all_players
    if msg_type == MSG_TYPE_POS:
        # 清空旧帧数据
        all_players.clear()

        for i in range(0, len(body_data), PLAYER_SIZE):
            # 截取单个玩家的20字节
            player_bytes = body_data[i:i+PLAYER_SIZE]

            # 安全判断：不足20字节直接跳过
            if len(player_bytes) != PLAYER_SIZE:
                continue

            # 解析单个玩家
            player_id, x, y = struct.unpack("<Idd", player_bytes)
            all_players[player_id] = {"X": x, "Y": y}


if connected:
    threading.Thread(target=receive_server_data, daemon=True).start()

while True:
    for event in pygame.event.get():
        if event.type == pygame.QUIT:
            pygame.quit()
            sys.exit()
        elif event.type == pygame.KEYDOWN:  # 空格清空轨迹
            if event.key == pygame.K_SPACE:
                ball.trail.clear()

    ball.trail.record_per_frame()

    keys = pygame.key.get_pressed()

    ball.cur_speed = ball.init_speed * \
        1.5 if (keys[pygame.K_LSHIFT] or keys[pygame.K_RSHIFT]
                ) else ball.init_speed

    if keys[pygame.K_LEFT]:
        ball.x = max(ball.x - ball.cur_speed, ball.r)
    if keys[pygame.K_RIGHT]:
        ball.x = min(ball.x + ball.cur_speed,
                     WIDTH - ball.r - ball.cur_speed)
    if keys[pygame.K_UP]:
        ball.y = max(ball.y - ball.cur_speed, ball.r)
    if keys[pygame.K_DOWN]:
        ball.y = min(ball.y + ball.cur_speed,
                     HEIGHT - ball.r - ball.cur_speed)

    screen.fill((0, 0, 0))

    # 坐标文本和模式提示的字体
    font = pygame.font.Font(None, 36)

    # 显示模式提示
    mode_text = "Single-Player Mode" if not connected else "Multiplayer Mode"
    mode_surface = font.render(mode_text, True, (100, 100, 100))
    screen.blit(mode_surface, (WIDTH - 600, 10))

    # 多人模式：显示其他玩家
    if connected:
        for idx, (pl_id, pos) in enumerate(all_players.items()):
            pygame.draw.circle(screen, (0, 255, 0),
                               (pos["X"], pos["Y"]), 20)

    pygame.draw.circle(screen, ball.color, (ball.x, ball.y), ball.r)  # 自己在最上层
    # 轨迹（颜色渐变效果）
    ball.trail.draw_gradient(screen)
    # 坐标文本
    pos_text = f"X: {ball.x:.2f}, Y: {ball.y:.2f} \n"
    text = font.render(
        pos_text, True, (255, 255, 255))

    # 发送位置（仅在连接时）
    if connected:
        cilent_socket.send(
            (json.dumps({"X": ball.x, "Y": ball.y})+"\n").encode("utf-8"))

    screen.blit(text, (10, 10))

    pygame.display.flip()
    clock.tick(64)

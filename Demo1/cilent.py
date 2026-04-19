# Pygame客户端
import pygame
import sys
import socket
import json
import time
import threading
from collections import deque
import struct

# =========================常量和初始化=========================

pygame.init()

WIDTH, HEIGHT = 1280, 800

HEADER_SIZE = 3  # 1字节类型+2字节长度

# 向服务端发送的消息的相关常量
MSG_TYPE_ENTER = 1  # 加入游戏消息
MSG_SIZE_ENTER = 0  # 加入游戏消息体长度为0

MSG_TYPE_MOVE = 2  # 小球移动消息
MSG_SIZE_MOVE = 1  # 小球移动消息体长度为1字节，1字节移动方向

# 接受的服务端消息相关常量
SERVER_MSG_TYPE_POS = 1  # 位置同步消息
PLAYER_SIZE = 6  # 每个玩家6字节

SERVER_MSG_TYPE_ENTER = 2  # 加入游戏消息

screen = pygame.display.set_mode((WIDTH, HEIGHT))
pygame.display.set_caption("Demo1-Cilent")


# =========================类定义=========================
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


# =========================全局变量=========================
# 线程锁
player_mutex = threading.Lock()
recv_msg_count_lock = threading.Lock()

# 用于粗略估计性能的变量
recv_msg_count = 0  # 每秒接收消息数
recv_msg_byte_count = 0  # 每秒接收消息字节数
cur_max_player_id = 0  # 当前最大玩家ID

display_msg_count = 0  # 显示的消息数
display_byte_count = 0  # 显示的字节数

# 所有玩家(ID:(X,Y))
all_players = {}

# 时钟
clock = pygame.time.Clock()

# 自己小球
ball = Ball(WIDTH // 2, HEIGHT // 2, 20, (255, 0, 0), 5)
self_player_id = None

last_msg_update_time = time.time()  # 上次统计消息数的时间
info_font = pygame.font.Font(None, 36)  # 预创建字体，优化性能

# 网络连接相关变量
cilent_socket = None
connected = False
# =========================网络连接处理=========================

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
    """接收服务器数据"""
    global recv_msg_count, recv_msg_byte_count, connected
    recv_buffer = bytearray()
    while True:
        if not connected:
            break
        try:
            data = cilent_socket.recv(16384)
            if not data:
                print("Server closed connection.")
                connected = False
                continue
            recv_buffer.extend(data)
            # 读包头
            while len(recv_buffer) >= HEADER_SIZE:
                # 取包头字节(1+2)
                header = recv_buffer[:HEADER_SIZE]
                msg_type, body_len = struct.unpack("<BH", header)

                # 总数据长度
                full_packet_len = HEADER_SIZE + body_len

                # 检验半包
                if len(recv_buffer) < full_packet_len:
                    continue
                # 取包
                full_packet = recv_buffer[:full_packet_len]
                del recv_buffer[:full_packet_len]

                # 提取消息体
                body_data = full_packet[HEADER_SIZE:]

                # 分配给处理函数
                handle_server_message(msg_type, body_data)
                # 统计接收消息数
                with recv_msg_count_lock:
                    recv_msg_count += 1
                    recv_msg_byte_count += full_packet_len

        except Exception as e:
            print("Receive server error:", e)
            break


def handle_server_message(msg_type, body_data):
    if msg_type == SERVER_MSG_TYPE_POS:
        with player_mutex:
            # 清空旧帧数据
            all_players.clear()
            for i in range(0, len(body_data), PLAYER_SIZE):
                player_bytes = body_data[i:i+PLAYER_SIZE]
                if len(player_bytes) != PLAYER_SIZE:
                    continue
                player_id, x, y = struct.unpack("<HHH", player_bytes)
                all_players[player_id] = {"X": float(x)/10, "Y": float(y)/10}
    elif msg_type == SERVER_MSG_TYPE_ENTER:
        # - msgtype = 2为玩家进入
        # 消息体为：
        # - 玩家ID uint16 2字节
        # - 是否是客户端自身的加入 bool 1字节
        # - X轴坐标 uint16 2字节
        # - Y轴坐标 uint16 2字节
        # 消息体长:7字节
        global self_player_id, ball
        # 截取2字节ID数据
        player_id = struct.unpack("<H", body_data[:2])[0]
        # 看是否是自身,是就记录
        is_self_enter = struct.unpack("<B", body_data[2:3])[0]
        # 记录坐标
        x, y = struct.unpack("<HH", body_data[3:7])
        x = float(x)/10
        y = float(y)/10
        if is_self_enter:
            self_player_id = player_id
            ball.x = x
            ball.y = y
        # 否则的情况我们先不做，等后续再处理


if connected:
    threading.Thread(target=receive_server_data, daemon=True).start()


# =========================主循环=========================
while True:
    # 如果没有自己的ID，发送进入的消息，然后等待服务器分配
    if self_player_id is None:
        cilent_socket.sendall(struct.pack(
            "<BH", MSG_TYPE_ENTER, MSG_SIZE_ENTER))
        time.sleep(1)
    if self_player_id is None:
        exit(1)

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

    is_accel = True if (keys[pygame.K_LSHIFT]
                        or keys[pygame.K_RSHIFT]) else False
    move_direction = 0
    if keys[pygame.K_LEFT]:
        move_direction = 3
        ball.x = max(ball.x - ball.cur_speed, ball.r)
    elif keys[pygame.K_RIGHT]:
        move_direction = 4
        ball.x = min(ball.x + ball.cur_speed, WIDTH - ball.r)
    if keys[pygame.K_UP]:
        move_direction += 2 if move_direction else 1
        ball.y = max(ball.y - ball.cur_speed, ball.r)
    elif keys[pygame.K_DOWN]:
        move_direction += 4 if move_direction else 2
        ball.y = min(ball.y + ball.cur_speed, HEIGHT - ball.r)
    if is_accel and move_direction:
        move_direction += 10

    screen.fill((0, 0, 0))

    # ===================== 统计 =====================
    current_time = time.time()
    # 每1秒更新一次接收消息数
    if current_time - last_msg_update_time >= 1.0:
        with recv_msg_count_lock:
            display_msg_count = recv_msg_count
            display_byte_count = recv_msg_byte_count
            recv_msg_count = 0
            recv_msg_byte_count = 0
        last_msg_update_time = current_time
    # 获取实时FPS
    current_fps = clock.get_fps()
    # ================================================================

    # 显示模式提示
    mode_text = "Single-Player Mode" if not connected else "Multiplayer Mode"
    mode_surface = info_font.render(mode_text, True, (100, 100, 100))
    screen.blit(mode_surface, (WIDTH - 600, 10))

    # 多人模式：显示其他玩家
    if connected:
        with player_mutex:
            players_list = list(all_players.items())

        for idx, (pl_id, pos) in enumerate(players_list):
            if pl_id == self_player_id:
                color = (255, 255, 0)
            else:
                color = (0, 255, 0)
            pygame.draw.circle(screen, color, (pos["X"], pos["Y"]), 20)
            player_id_surface = info_font.render(
                str(pl_id), True, (64, 188, 75))
            screen.blit(player_id_surface, (pos["X"] - 10, pos["Y"] - 10))

    # 玩家自身的半透明显示
    # 创建带透明通道的临时表面
    circle_surf = pygame.Surface((ball.r*2, ball.r*2), pygame.SRCALPHA)
    # 绘制半透明圆，最后一个参数是透明度
    pygame.draw.circle(circle_surf, (*ball.color, 80),
                       (ball.r, ball.r), ball.r)
    # 将半透明圆贴到屏幕上
    screen.blit(circle_surf, (ball.x - ball.r, ball.y - ball.r))

    ball.trail.draw_gradient(screen)

    # 坐标文本
    pos_text = f"X: {ball.x:.2f}, Y: {ball.y:.2f}"
    text = info_font.render(pos_text, True, (255, 255, 255))
    screen.blit(text, (10, 10))

    # 显示当前玩家数量
    if connected:
        with player_mutex:
            player_count = len(all_players)
            cur_max_player_id = max(all_players.keys())
        player_count_text = f"Players:{player_count} - Max ID:{cur_max_player_id}"
        player_count_surface = info_font.render(
            player_count_text, True, (255, 255, 255))
        screen.blit(player_count_surface, (10, 50))

    # 渲染FPS（青色）
    fps_surface = info_font.render(
        f"FPS: {current_fps:.1f}", True, (0, 255, 255))
    screen.blit(fps_surface, (10, 90))
    # 渲染每秒接收消息数（绿色）
    msg_surface = info_font.render(
        f"Recv Msg/s: {display_msg_count}", True, (0, 255, 0))
    screen.blit(msg_surface, (10, 130))
    # 渲染每秒接收字节数（黄色）
    byte_surface = info_font.render(
        f"Recv Byte/s: {display_byte_count} (~{display_byte_count/60:.2f}B/pkg)", True, (255, 255, 0))
    screen.blit(byte_surface, (10, 170))

    # 发送位置（仅在连接时）
    if connected and move_direction:
        bin_data = struct.pack("<BHB", MSG_TYPE_MOVE,
                               MSG_SIZE_MOVE, move_direction)
        cilent_socket.send(bin_data)

    pygame.display.flip()
    clock.tick(64)

import socket
import threading
import random
import time
import struct

# ===================== 配置常量（与客户端完全一致）=====================
SERVER_IP = "127.0.0.1"
SERVER_PORT = 16543
SIMULATE_COUNT = 180  # 模拟客户端数量
WIDTH, HEIGHT = 1280, 800
TICK_RATE = 64  # 同步帧率

# 协议常量
HEADER_SIZE = 3
MSG_TYPE_ENTER = 1   # 加入游戏
MSG_SIZE_ENTER = 0
MSG_TYPE_MOVE = 2    # 小球移动
MSG_SIZE_MOVE = 1

# ===================== 模拟客户端逻辑 =====================


def simulate_ball(client_id):
    try:
        # 创建TCP连接
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        sock.connect((SERVER_IP, SERVER_PORT))
        sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)  # 低延迟

        # 1. 【必须】发送加入游戏消息
        enter_packet = struct.pack("<BH", MSG_TYPE_ENTER, MSG_SIZE_ENTER)
        sock.sendall(enter_packet)

        # 随机初始化小球参数
        x = random.randint(50, WIDTH - 50)
        y = random.randint(50, HEIGHT - 50)
        base_speed = random.randint(2, 6)

        while True:
            # 2. 生成随机移动方向（完全匹配客户端逻辑）
            move_direction = 0
            is_accel = random.choice([True, False])  # 随机加速

            # 随机基础方向：0不动 / 1上 2下 3左 4右 / 5上左 6上右 7下左 8下右
            base_dir = random.choice([0, 1, 2, 3, 4, 5, 6, 7, 8])
            if base_dir != 0:
                move_direction = base_dir
                # 加速则方向+10
                if is_accel:
                    move_direction += 10

            # 3. 根据方向更新坐标（边界校验）
            speed = base_speed * 1.5 if is_accel else base_speed
            if move_direction in (1, 5, 6, 11, 15, 16):  # 上
                y = max(y - speed, 20)
            if move_direction in (2, 7, 8, 12, 17, 18):  # 下
                y = min(y + speed, HEIGHT - 20)
            if move_direction in (3, 5, 7, 13, 15, 17):  # 左
                x = max(x - speed, 20)
            if move_direction in (4, 6, 8, 14, 16, 18):  # 右
                x = min(x + speed, WIDTH - 20)

            # 4. 【核心】按协议发送移动消息
            move_packet = struct.pack(
                "<BHB", MSG_TYPE_MOVE, MSG_SIZE_MOVE, move_direction)
            sock.sendall(move_packet)

            # 控制帧率
            time.sleep(1 / TICK_RATE)

            # 清空服务端广播数据，避免缓冲区积压
            try:
                sock.setblocking(False)
                while sock.recv(4096):
                    pass
                sock.setblocking(True)
            except BlockingIOError:
                pass

    except Exception as e:
        print(f"客户端{client_id} 断开: {str(e)[:50]}")
    finally:
        sock.close()


# ===================== 启动压测 =====================
if __name__ == "__main__":
    print(f"=== 启动 {SIMULATE_COUNT} 个协议适配压测客户端 ===")
    for i in range(SIMULATE_COUNT):
        threading.Thread(target=simulate_ball, args=(i,), daemon=True).start()
        time.sleep(0.001)  # 避免瞬间连接风暴

    print("压测运行中... 按 Ctrl+C 停止")
    while True:
        time.sleep(1)

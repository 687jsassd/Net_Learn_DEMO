import socket
import threading
import random
import time
import struct  # 新增：用于二进制打包

# 配置（和你的客户端/服务端完全一致）
SERVER_IP = "127.0.0.1"
SERVER_PORT = 16543
SIMULATE_COUNT = 500
WIDTH, HEIGHT = 1280, 800
TICK_RATE = 64  # 64帧同步


def simulate_ball(client_id):
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        sock.connect((SERVER_IP, SERVER_PORT))
        sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)

        # 随机初始化坐标
        x = random.randint(50, WIDTH - 50)
        y = random.randint(50, HEIGHT - 50)
        speed = random.randint(2, 11)

        while True:
            # 随机移动逻辑（和客户端行为一致）
            x += random.choice([-speed, 0, speed])
            y += random.choice([-speed, 0, speed])
            x = max(20, min(x, WIDTH - 20))
            y = max(20, min(y, HEIGHT - 20))

            # ==============================================
            # 核心修改：二进制打包（和客户端完全一样！）
            # 坐标 *10 转uint16，小端序打包 <HH
            x_int = int(round(x * 10))
            y_int = int(round(y * 10))
            bin_data = struct.pack("<HH", x_int, y_int)
            # 发送二进制数据（无JSON、无换行）
            sock.sendall(bin_data)
            # ==============================================

            time.sleep(1 / TICK_RATE)

            # 清空服务端广播的接收缓冲区（避免积压）
            try:
                sock.setblocking(False)
                while sock.recv(4096):
                    pass
                sock.setblocking(True)
            except BlockingIOError:
                pass

    except Exception as e:
        print(f"客户端{client_id}异常: {e}")
    finally:
        sock.close()


if __name__ == "__main__":
    print(f"启动 {SIMULATE_COUNT} 个二进制协议压测客户端...")
    for i in range(SIMULATE_COUNT):
        threading.Thread(target=simulate_ball, args=(i,), daemon=True).start()
        time.sleep(1 / TICK_RATE)

    print("压测运行中...")
    while True:
        time.sleep(1)

import socket
import json
import threading
import random
import time

# 配置
SERVER_IP = "127.0.0.1"
SERVER_PORT = 16543
SIMULATE_COUNT = 500
WIDTH, HEIGHT = 1280, 800


def simulate_ball(client_id):
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        sock.connect((SERVER_IP, SERVER_PORT))
        sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)

        x = random.randint(50, WIDTH-50)
        y = random.randint(50, HEIGHT-50)
        speed = random.randint(2, 11)

        while True:
            # 随机移动
            x += random.choice([-speed, 0, speed])
            y += random.choice([-speed, 0, speed])
            x = max(20, min(x, WIDTH-20))
            y = max(20, min(y, HEIGHT-20))

            msg = json.dumps({"x": x, "y": y}) + "\n"
            sock.sendall(msg.encode())
            time.sleep(1/64)  # 模拟64tick

            # ✅ 极简清空二进制接收缓冲区（适配新包头协议）
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
    print(f"启动 {SIMULATE_COUNT} 个压测客户端...")
    for i in range(SIMULATE_COUNT):
        threading.Thread(target=simulate_ball, args=(i,), daemon=True).start()
        time.sleep(1/64)

    print("压测运行中...")
    while True:
        time.sleep(1)

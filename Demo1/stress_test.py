import socket
import json
import threading
import random
import time

# 压力测试配置
SERVER_IP = "127.0.0.1"
SERVER_PORT = 16543
SIMULATE_COUNT = 50
WIDTH, HEIGHT = 1280, 800

# 单个模拟客户端


def simulate_ball():
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.connect((SERVER_IP, SERVER_PORT))
        # 随机初始位置
        x = random.randint(50, WIDTH-50)
        y = random.randint(50, HEIGHT-50)
        speed = random.randint(2, 11)

        while True:
            # 随机移动
            x += random.choice([-speed, 0, speed])
            y += random.choice([-speed, 0, speed])
            # 边界限制
            x = max(20, min(x, WIDTH-20))
            y = max(20, min(y, HEIGHT-20))
            # 发送坐标
            msg = json.dumps({"X": x, "Y": y}) + "\n"
            sock.send(msg.encode())
            time.sleep(0.03)  # 30ms同步一次

            # 接收服务器响应，不处理，防止因接收问题导致的服务端阻塞
            data = sock.recv(4096).decode("utf-8")

    except:
        pass


# 批量启动模拟客户端
if __name__ == "__main__":
    print(f"启动 {SIMULATE_COUNT} 个模拟小球...")
    for i in range(SIMULATE_COUNT):
        t = threading.Thread(target=simulate_ball, daemon=True)
        t.start()
        # 防止连接风暴，轻微延迟
        if i % 100 == 0:
            time.sleep(0.1)
    # 保持程序运行
    while True:
        time.sleep(1)

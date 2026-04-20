import socket
import threading
import random
import time
import struct

# ===================== 配置常量（与客户端100%对齐）=====================
SERVER_IP = "127.0.0.1"
SERVER_PORT = 16543
SIMULATE_COUNT = 333   # 模拟客户端数量
WIDTH, HEIGHT = 1280, 800
TICK_RATE = 64         # 同步帧率

# 协议核心常量
HEADER_SIZE = 3                # 固定包头：1字节类型 + 2字节长度
AUTH_NEW_CLIENT = 0x01         # 新客户端认证
AUTH_OLD_CLIENT = 0x02         # 旧客户端认证

# 客户端发送的消息
MSG_TYPE_ENTER = 1             # 加入游戏
MSG_SIZE_ENTER = 0
MSG_TYPE_MOVE = 2              # 小球移动
MSG_SIZE_MOVE = 3              # 消息体：2字节ID + 1字节方向

# 服务端返回的消息（新增！必须解析）
SERVER_MSG_TYPE_POS = 1        # 位置同步
SERVER_MSG_TYPE_ENTER = 2      # 玩家进入（分配ID）
PLAYER_ENTER_BODY_LEN = 7      # 服务端ENTER消息体长度：7字节

# ===================== 模拟客户端逻辑 =====================


def simulate_ball():
    """模拟单个游戏客户端：自动接收服务端分配的ID，合法发送消息"""
    sock = None
    recv_buffer = bytearray()  # 接收缓冲区（和客户端一致）
    self_player_id = 0         # 服务端动态分配的真实ID（核心变量）
    try:
        # 1. 创建TCP连接
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(10)
        sock.connect((SERVER_IP, SERVER_PORT))
        sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)

        # 2. 发送【新客户端认证】（客户端原生逻辑：新客户端ID传0）
        auth_packet = struct.pack("<BH", AUTH_NEW_CLIENT, 0)
        sock.sendall(auth_packet)
        time.sleep(0.01)

        # 3. 发送【加入游戏消息】
        enter_packet = struct.pack("<BH", MSG_TYPE_ENTER, MSG_SIZE_ENTER)
        sock.sendall(enter_packet)

        # ============== 核心：等待服务端分配角色ID ==============
        while self_player_id == 0:
            data = sock.recv(1024)
            if not data:
                raise Exception("服务端断开连接，未分配ID")
            recv_buffer.extend(data)

            # 解析包头（和客户端完全一致）
            while len(recv_buffer) >= HEADER_SIZE:
                header = recv_buffer[:HEADER_SIZE]
                msg_type, body_len = struct.unpack("<BH", header)
                full_packet_len = HEADER_SIZE + body_len

                if len(recv_buffer) < full_packet_len:
                    break
                # 提取完整数据包
                full_packet = recv_buffer[:full_packet_len]
                del recv_buffer[:full_packet_len]
                body_data = full_packet[HEADER_SIZE:]

                # 解析服务端分配的ID（关键！）
                if msg_type == SERVER_MSG_TYPE_ENTER:
                    player_id = struct.unpack("<H", body_data[:2])[0]
                    is_self_enter = struct.unpack("<B", body_data[2:3])[0]
                    if is_self_enter:
                        self_player_id = player_id
                        print(f"✅ 客户端获取服务端分配ID: {self_player_id}")

        # 随机初始化小球参数
        x = random.randint(50, WIDTH - 50)
        y = random.randint(50, HEIGHT - 50)
        base_speed = random.randint(2, 6)

        # ============== 开始合法发送移动消息 ==============
        while True:
            # 生成随机移动方向（匹配客户端）
            move_direction = 0
            is_accel = random.choice([True, False])
            base_dir = random.choice([0, 1, 2, 3, 4, 5, 6, 7, 8])

            if base_dir != 0:
                move_direction = base_dir
                if is_accel:
                    move_direction += 10

            # 发送移动消息：使用【服务端分配的真实ID】
            move_packet = struct.pack(
                "<BHHB",
                MSG_TYPE_MOVE,
                MSG_SIZE_MOVE,
                self_player_id,  # 用真实ID，服务端校验通过
                move_direction
            )
            sock.sendall(move_packet)

            # 控制帧率
            time.sleep(1 / TICK_RATE)

            # 清空服务端广播数据，避免缓冲区积压
            try:
                sock.setblocking(False)
                while sock.recv(16384):
                    pass
                sock.setblocking(True)
            except BlockingIOError:
                pass

    except Exception as e:
        print(f"❌ 客户端断开 (ID:{self_player_id}): {str(e)[:40]}")
    finally:
        if sock:
            sock.close()


# ===================== 启动压测 =====================
if __name__ == "__main__":
    print(f"=== 启动 {SIMULATE_COUNT} 个压测客户端（自动获取服务端ID）===")
    print("✅ 修复核心问题：自动解析服务端分配的角色ID，消息合法可通过校验")

    # 启动多线程压测（无需手动传ID，服务端动态分配）
    for _ in range(SIMULATE_COUNT):
        threading.Thread(target=simulate_ball, daemon=True).start()
        time.sleep(0.005)

    print("压测运行中... 按 Ctrl+C 停止")
    while True:
        time.sleep(1)

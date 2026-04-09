
---
# Init
## 修改目标：抗住500球

# 目前问题：
1. 单协程负责收集+json序列化+分发，锁占用时间长，职责不明确。
2. json序列化是严重CPU密集型操作，扛不住负载。
3. 分发是串行的，慢。
4. 慢客户端阻塞整体，不稳定。
5. Ball结构体太简单，用对象池反而差。
6. Ball的float64的X和Y在32位上不是原子操作，且需要依次修改两个值，一定不原子，需要加锁。


# 方案：独立化多点分发+舍弃json改用二进制序列化+加锁保护+其他优化。

1. 每个连接创建写协程，不用单广播进行分发写入
2. 广播改为收集状态，发送给序列化-分发协程，该协程分发给每个连接的通道。
3. 客户端的显示帧率和发送频率分离，不然发包太多。
4. 弃用对象池。
5. 加锁保护Ball的写入，使用atomic.Float64进行初步尝试。
6. 换协议,不用json,用自定义二进制协议搞。



# 实现过程中问题
---
- 对方案5的实现过程
1. atomic.Float64目前不存在，我改用atomic.Uint64,写操作下需要把float64使用math.Float64bits转换为uint64。读操作下需要使用math.Float64frombits转换为float64。 
    *这个操作性能开销大吗？

2. 为了方便识别，我给每个球分配了一个uint32的ID，这样能够避免直接传送IP地址，提高效率。
    *但是原客户端是通过self_addr = cilent_socket.getsockname()获取IP地址，进而用于识别自己的，
    现在如何修改？
    可能需要服务端一个接口，用于客户端获取自己的ID？

3. 我为结构体Ball添加了SetXY方法，用于设置球的位置。这里使用了Store方法，确保原子性。
    但是，SetXY方法需要依次修改两个值，此时还原子吗？
    显然不是，我们需要加锁保护。使用sync.Mutex。

- 对方案1的实现过程
1. 如果每个业务操作都要一个chan，性能开销会比较大？
   对二进制协议，由于有4字节的业务代号字段，可以通过识别它+switch-case实现分发，但是这样好吗？

- 对方案2的实现过程
1.  ```
    clients_nums := len(cilents) //收集一下数量，后面分配切片大小直接分配
    if clients_nums == 0 {
        cilents_Mu.RUnlock()
        continue
    }
    //快速收集拷贝一遍所有客户端的数据
    var positions []struct {
        X, Y float64
        ID   uint32
    } = make([]struct {
        X, Y float64
        ID   uint32
    }, clients_nums)
    ```

    这里是否有问题？导致切片默认分配clients_nums个元素，但是又跳过了ID=0的元素，导致切片存在0值？？
    这会导致客户端收到的坐标错误（额外有0值小球）


现在我把服务端广播间隔设为了16ms，近似模拟64tick服务器。
在100球时，完全流畅；在500球时，客户端存在闪烁问题，但服务端正常，目标完成。



--- 

# 26.04.17

## 目标：简化包体大小(当前数据类型选定不佳)
我们发现：
``` 
type BallObj struct {
	X  atomic.Uint64
	Y  atomic.Uint64
	ID uint32 //分配一个球ID，方便识别，而且避免传输时直接传送IP地址
	mu sync.Mutex
} 

 func (b *BallObj) GetXY() (float64, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return math.Float64frombits(b.X.Load()), math.Float64frombits(b.Y.Load())
}
func (b *BallObj) SetXY(x, y float64) { //虽然单个是原子操作，但是俩就不是了，得加锁！
	b.mu.Lock()
	defer b.mu.Unlock()
	b.X.Store(math.Float64bits(x))
	b.Y.Store(math.Float64bits(y))
} 
```
这里存在一定设计问题:
- atomic是为了解决单个字段的无锁并发读写安全。
- 但是这里需要依次修改两个值，本身这个操作是要求原子的，但atomic不能保证两个顺序操作的原子性，因此我们引入了锁
- 但是引入锁之后，其内的操作就是原子的了，无需atomic再进行保护，所以这里有冗余问题！

我们舍弃atomic，为了简化包体考虑，我们使用uint16

uint16范围为0-2e16 = 0 - 65535，
有如下考虑：
- 使用乘除10来表示客户端传来的float，我们只需要一位精度。
- 考虑到客户端坐标一般不可能达到6553(比如demo是1280*720)，其最大坐标值为1280.0，我们的设计有足够空间表示。
- uint16占用2字节，比uint64/float64的8字节要小得多。

对于ID也用uint16，因为这是同屏的玩家动态分配的ID，
一般不会超过65535，其实也可以引入类似池的概念进行ID复用，总体ID绝对足够。

对于传输协议上，我们直接传输uint16即可
在客户端进行转换，避免服务端开销。

对于包头，我们用uint8(0-255)表示业务代号，因为大类消息不会很多。
用uint16表示消息长度。
总体占用3字节，比之前8字节节省很多。

总包大小：
包头 3 + 消息体 6n 




### 对于ID问题
之前使用atomic.Uint32,但有无必要？
如果有很多客户端同时连接，可能产生竞态，因此选型正确
但是我们现在要改uint16,而atomic.Uint16不存在，所以手动引入锁。

### python解析时struct.unpack相关参数问题

| Go 语言类型 | 字节数 | Python struct 格式符 | 含义 |
|---|---|---|---|
| uint8 | 1 字节 | B | 无符号小整数（0~255） |
| uint16 | 2 字节 | H | 无符号中整数（0~65535） |
| uint32 | 4 字节 | I | 无符号大整数 |
| uint64 | 8 字节 | Q | 无符号超大整数 |
| int8 | 1 字节 | b | 有符号小整数（-128~127） |
| int16 | 2 字节 | h | 有符号中整数（-32768~32767） |
| int32 | 4 字节 | i | 有符号大整数 |
| int64 | 8 字节 | q | 有符号超大整数 |
| float32 | 4 字节 | f | 单精度浮点数 |
| float64 | 8 字节 | d | 双精度浮点数 |
| bool | 1 字节 | ? | 布尔值（True/False） |
| byte | 1 字节 | B | 与 uint8 等价（0~255） |
| uint | 4 或 8 字节（取决于系统） | I 或 Q | 无符号整数（位数取决于系统架构） |
| int | 4 或 8 字节（取决于系统） | i 或 q | 有符号整数（位数取决于系统架构） |
| uintptr | 4 或 8 字节（取决于系统） | I 或 Q | 指针值（无符号整数形式，位数取决于系统架构） |

### 客户端发送问题
目前客户端还用的是json，服务端需要使用json.Unmarshal解析
毕竟我们要发uint16了，不如客户端也用二进制协议。
修改客户端和服务端相关内容即可。

---
##  我完成了修改
目前500球下流畅，客户端也没有闪烁情况。  
---


---
## 26.04.08
问题：目前使用状态同步，为学习指令同步，我决定改为指令同步。
指令同步会把玩家A的指令推送给所有其他玩家，然后其他玩家根据指令更新自己的状态。无需服务器进行状态更新。

目标：
完成服务端的接收消息->解析->丢给战局内队列->完成每帧处理消息-广播-tick的同步循环。

规定：
- 客户端->服务端消息格式:

    - msgtype = 1为玩家进入
        消息体为空
        消息体长:0字节

    - msgtype = 2为小球移动
        消息体包括：
        - X轴速度 int8(-128-127) 1字节 
        - Y轴速度 int8(-128-127) 1字节 
        (似乎也可以用轴坐标来表示，应该也是2字节足够)
        消息体长:2字节
    
    - msgtype = 3为小球位置同步(等待弃用，只是中途用)
        消息体包括：
        - X轴坐标 uint16 2字节
        - Y轴坐标 uint16 2字节
        消息体长:4字节

- 服务端->客户端消息格式:

    - msgtype = 2为玩家进入
        消息体为：
        - 玩家ID uint16 2字节
        - 是否是客户端自身的加入 bool 1字节
        - X轴坐标 uint16 2字节
        - Y轴坐标 uint16 2字节
        消息体长:7字节

    - msgtype = 3为小球移动
        消息体包括：
        - 玩家ID uint16 2字节
        - X轴速度 int8(-128-127) 1字节 
        - Y轴速度 int8(-128-127) 1字节 
        消息体长:4字节


因为此时服务器只负责转发即可，无需进行状态更新，我们可以把Ball等结构体直接舍弃。

服务端建立一个帧操作队列(chan)，用于接收客户端发送的指令。
随后，把指令从队列中取出，广播给所有其他玩家。
客户端可以设置一个检测，检查指令是否由自己发起，如果是那就忽略掉。

客户端进入->服务端接受->返回ID->客户端存储ID用于检测



## 关于消息分发
我们目前已经对每个客户端建立一个goroutine进行从写通道到实际写入链接的写入，
但目前没有一个方便的函数用于将消息分发给指定客户端的写通道。
鉴于无论是状态同步还是指令同步，都离不开全量广播，因此这个函数尤为重要，复用性很高。

下面的spread_positions函数已经初见端倪，我们可以进行修改以适配需要。
```
func spread_positions() {
	for binData := range broadcast_chan {
		writing_chans_Mu.Lock()
		for conn, ch := range writing_chans {
			select {
			case ch <- binData:
			default:
				fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
			}
		}
		writing_chans_Mu.Unlock()
	}
}
```
在目前的实现中，由于直接从broadcast_chan中取出消息，其类型为[]byte,没有ID以进行识别
所以不方便做到剔除客户端自身的消息。
这可以在客户端中进行过滤，因为消息本身带有ID。
- 若选择从消息中读取ID，是否慢？？不慢的话可能可行。
- 认为，二进制解析非常快，不会影响性能，因此我们可以直接从消息中读取ID。

## 关于服务端读取时缓冲区大小选取的重要问题
```
	// 接收消息，开始解析
	buf := make([]byte, 1024)
	for {
		//读1字节uint8的消息类型
		n, err := io.ReadFull(conn, buf[0:1])
		if err != nil || n != 1 {
			return
		}
		msgType := buf[0]
		//读2字节uint16的消息长度
		n, err = io.ReadFull(conn, buf[1:3])
		if err != nil || n != 2 {
			return
		}
		msgLen := binary.LittleEndian.Uint16(buf[1:3])
		//剩下的是消息体
		msgData := buf[3:]
		fmt.Println("Received message:", msgType, msgLen, msgData)
		//检查消息体长度是否正确
		if uint16(len(msgData)) != msgLen {
			fmt.Println("Invalid message length:", msgType, msgLen, msgData)
			return
		}
		//构造InGameMsg,送chan
		msg := InGameMsg{
			Conn:    conn,
			MsgType: msgType,
			MsgData: msgData,
		}
		//放不进去就不放了，算丢弃
		select {
		case ingame_process_msg_chan <- msg:
		default:
		}
	}
```
这样的代码是不对的，因为buf建立了1024字节的固定大小切片，
使用其进行读取时，其消息体必然是1024-3 = 1021字节。
重要：这里切片不会自动缩容，因此使用msgLen的比较必然无效。
- 服务端必须严格按照 msgLen 读取 exactly N 字节的消息体，而不是直接用切片剩余空间！

## 对于在严格帧同步(每帧处理一次-发送一次消息)时消息分发函数的阻塞问题
```
func write_chan_msg_for_all_clients() {
	//这是通用的全量广播函数，不作区分
	for binData := range broadcast_chan {
		writing_chans_Mu.RLock()
		for conn, ch := range writing_chans {
			select {
			case ch <- binData:
			default:
				fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
			}
		}
		writing_chans_Mu.RUnlock()
	}
}
```
上面的函数适合单独开协程运行，不可以在帧同步中调用。
原因是，for binData := range broadcast_chan该条件在broadcast_chan为空时，会阻塞。
由于不是每次都有消息，所以该写法会导致严重问题。
可以使用for+ select来实现非阻塞的读取,注意for必须有限次数，否则会导致死循环。
这里我for次数改为了与broadcast_chan的容量相同，即125。


## 关于Go写入binData必须加类型的问题
_ = binary.Write(buf_other, binary.LittleEndian, uint16(640))
以上是正确的
但是
_ = binary.Write(buf_other, binary.LittleEndian, 640)
这样的是错误的，因为具体类型未知

binary.Write 要求传入固定大小、明确类型的数据，不能传入无类型的数字字面量（如 640），也不能传入平台相关的 int/uint 类型。
你必须显式指定类型（如 uint16(640)），否则函数无法确定要写入多少字节的二进制数据。

// 标准库 encoding/binary 包
func Write(w io.Writer, order ByteOrder, data any) error
虽然data是接口，接受任意类型值，但是函数底层依赖反射，必须知道 data 的具体类型和固定字节长度，才能把数据转换成二进制字节。

如果你binary.Write(buf_other, binary.LittleEndian, 640)这么写，实际上640根本不会写入，但是也不报错。
需要特别注意这一点。

## 关于重复读锁(锁重入)错误
```
func write_message_for_client(conn net.Conn, binData []byte) {
	//这是通用的单客户端广播函数
	writing_chans_Mu.RLock()
	select {
	case writing_chans[conn] <- binData:
	default:
		fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
	}
	writing_chans_Mu.RUnlock()
}

func write_diff_msg_for_spc_client(spc_conn net.Conn, spc_binData []byte, other_binData []byte) {
	//为指定连接写指定消息，其他的写其他消息
	writing_chans_Mu.RLock()
	for conn := range writing_chans {
		if conn == spc_conn {
			write_message_for_client(conn, spc_binData)
			continue
		}
		write_message_for_client(conn, other_binData)
	}
	writing_chans_Mu.RUnlock()
}

```
上面的代码有重复读锁错误，其导致问题。
具体地：

TODO 待续 26.04.09

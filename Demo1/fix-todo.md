
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
问题：目前使用状态同步，为学习指令同步和客户端->服务端通讯，对目前的状态同步进行修改。
把玩家A的指令推送给所有其他玩家，然后其他玩家根据指令更新自己的状态。无需服务器进行状态更新。

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
Go 语言原生的 sync.Mutex（互斥锁）和 sync.RWMutex（读写锁）都是【不可重入锁】
如果同一个 goroutine 对同一把锁重复执行 Lock() 加锁操作，会直接触发死锁。

Java 的 ReentrantLock（可重入锁）会记录两个关键信息：
① 持有锁的线程 ID；② 锁的重入计数。
同一个线程再次加锁时，仅计数 + 1，不会阻塞。
Go 的 sync.Mutex 极简设计：
它只记录三个状态：
锁是否被持有；
等待锁的 goroutine 队列；
饥饿模式标记。
完全不存储「当前持有锁的 goroutine ID」，也没有「重入计数」。

在write_diff_msg_for_spc_client中，不能调用write_message_for_client，重新设计以解决该问题，只需要将操作逻辑展开即可。

---


# 我们先实现发送指令而非坐标的状态同步方式。

因此，    
- msgtype = 2为小球移动
        消息体包括：
        - X轴速度 int8(-128-127) 1字节 
        - Y轴速度 int8(-128-127) 1字节 
        (似乎也可以用轴坐标来表示，应该也是2字节足够)
        消息体长:2字节

我们改为
    - msgtype = 2为小球移动
        消息体包括：
		移动方式 uint8 1字节
        消息体长:1字节

		规定：
		- 1: 上
		- 2: 下
		- 3: 左
		- 4: 右
		- 5: 左上
		- 6: 右上
		- 7: 左下
		- 8: 右下
		- 11: 加速上
		- 12: 加速下
		- 13: 加速左
		- 14: 加速右
		- 15: 加速左上
		- 16: 加速右上
		- 17: 加速左下
		- 18: 加速右下

## 关于防溢出问题
在服务端权威结构中，我们需要进行坐标计算，但必须避免溢出情况
这里我采用了偏移量法，即每次计算坐标时，都先将坐标加一个偏移量1000，再进行计算。
计算后，进行边界检查，再将坐标减去偏移量1000，即可得到实际坐标。
下面的实现虽然正确可用，但是工程上并不美观，寻找其他更好的方法？
```
	case 2: //移动指令
		// - msgtype = 2为小球移动
		// 消息体包括：
		// - 移动方式 uint8 1字节
		// 消息体长:1字节

		//无需发送给客户端消息，这里根据收到的做同步即可
		move_direction := msg.MsgData[0]
		cilents_Mu.RLock()
		cur_x, cur_y := cilents[msg.Conn].GetXY()
		cilents_Mu.RUnlock()
		//因为uint16被减为0以下会导致溢出，为防溢出考虑，先均加1000，后续扣除
		cur_x += 1000
		cur_y += 1000
		switch move_direction {
		case 1: //上
			cur_y -= 50
		case 2: //下
			cur_y += 50
		case 3: //左
			cur_x -= 50
		case 4: //右
			cur_x += 50
		case 5: //左上
			cur_x -= 50
			cur_y -= 50
		case 6: //右上
			cur_x += 50
			cur_y -= 50
		case 7: //左下
			cur_x -= 50
			cur_y += 50
		case 8: //右下
			cur_x += 50
			cur_y += 50
		case 11: //加速上
			cur_y -= 100
		case 12: //加速下
			cur_y += 100
		case 13: //加速左
			cur_x -= 100
		case 14: //加速右
			cur_x += 100
		case 15: //加速左上
			cur_x -= 100
			cur_y -= 100
		case 16: //加速右上
			cur_x += 100
			cur_y -= 100
		case 17: //加速左下
			cur_x -= 100
			cur_y += 100
		case 18: //加速右下
			cur_x += 100
			cur_y += 100
		}
		//边界校验
		if cur_x < 1000+BallRadius {
			cur_x = 1000 + BallRadius
		}
		if cur_x > 1000+Width-BallRadius {
			cur_x = 1000 + Width - BallRadius
		}
		if cur_y < 1000+BallRadius {
			cur_y = 1000 + BallRadius
		}
		if cur_y > 1000+Height-BallRadius {
			cur_y = 1000 + Height - BallRadius
		}
		//扣除1000，然后设置
		cur_x -= 1000
		cur_y -= 1000

		cilents_Mu.Lock()
		cilents[msg.Conn].SetXY(cur_x, cur_y)
		cilents_Mu.Unlock()

```
一种考虑是，无符号整数天然弱减法运算，而使用有符号整数int16也可以支持我们目前的坐标范围
所以可以换int16，这样不需要特别处理溢出问题。
这是一个选型经验问题。

我们保留uint16，因为如果要变动数据类型，是牵一发而动全身的，我们可以整个函数来暂时用一下
因为目前我还没有写常用的实用函数库，所以暂时用函数来实现。
我们封装这样的函数来实现安全加减法：
```
SafeSub := func(x, n, min uint16) uint16 {
    if x <= n || x - n < min { 
        return min
    }
    return x - n
}
SafeAdd := func(x, n, max uint16) uint16 {
	if x+n > max {
		return max
	}
	return x + n
}
```
后续可以使用泛型来实现这样的便利。

- 注意！ SafeSub千万不要写为下面这样:
  ```
  SafeSub := func(x, n, min uint16) uint16 {
    if x - n < min { 
        return min
    }
    return x - n
	}
  ```
  因为比较中的x-n可能会溢出，导致结果错误。
  必须先判断x是否小于等于n，确保不会溢出再减。


## 关于每帧处理条数的限制以及通道缓冲区
在我们的循环中:
```
func ingame_process_loop() {
	//64tick处理一次
	//先处理消息，再收集位置，再广播，完成一次循环(1帧)
	for {
		time.Sleep(GameLoopInterval)

		// 只处理2000条消息，其他的下一帧再处理
		for i := 0; i < 2000; i++ {
			select {
			case msg := <-ingame_process_msg_chan:
				// 处理消息
				process_ingame_msg(msg)
			default:
			}
		}
		// 收集位置
		collect_positions()
		// 广播位置
		write_chan_msg_for_all_clients()
	}
}
```
拥有一个每帧处理条数限制，这里是2000，是修改后的
原先是125，但是发现，由于压力测试会带来500+小球，每个小球每帧都会至少一条移动指令，
所以一旦条数限制太小，就会导致处理速度跟不上，从而导致游戏延迟

缓冲区是另一个限制甚至导致错误的点。
`ingame_process_msg_chan = make(chan InGameMsg, 2000)`
这里的缓冲区一开始开的是125，面对超过125个小球时，由于我们对于缓冲区满时的接下来的消息的处理策略是直接舍弃，所以导致部分客户端的消息直接丢失，直接导致不同步异常。
从这里看到，缓冲区开小比每帧处理限制开小要更危险。

但是，缓冲区的大小，对于性能来说，有什么影响？？
- Go 通道的缓冲区是预分配的环形队列，底层实现极高效，这两个大小对服务器性能的影响可以忽略
  我们现在的InGameMsg结构体大小计算如下：
  net.Conn = Go 接口 → 固定占 16 字节（存两个指针：类型指针 + 数据指针）
  MsgType uint8 = 1 字节 → 内存对齐补 7 字节 → 占 8 字节
  MsgData []byte = 切片 → 固定占 24 字节（存三个值：数据指针 + 长度 + 容量）
  合计每个InGameMsg结构体的大小为48字节，即使开10W缓冲区，也才4.8MB，基本可以忽略不计。
  而通道访问是O(1);
  只要缓冲区没满，写入协程永远不会阻塞，Go 调度器开销不增加;
  缓冲区大小不会增加任何计算、循环、判断开销；
  通道缓冲区是连续内存块，Go GC 扫描成本极低，增加开销很小。
  总的来看，对硬件层面，性能影响几乎是忽略不计的
- 但是，在业务层面，不能无脑开大缓冲区，主要是因为，缓冲区可以积压消息，而一旦处理速度跟不上：
  - 如果缓冲区小，积压上限低，延迟可控，放不进去的直接丢就行；
  - 如果缓冲区大，积压上限高，延迟是不可控的，你永远在处理过时的旧消息，新消息永远排在后面，比丢包还可怕，直接玩不了。

### 一些其他备注：
- 一般来说用chan *InGameMsg更好，因为只存8字节指针，比我们现在的48字节内存省非常多；
  
  在绝大多数服务端、高并发、大 / 中型结构体场景下，通道存指针（chan *T）是主流、更优、最常用的写法；
  只有极小结构体 / 纯值类型场景，才会用通道存值（chan T）

  性能上，存值：通道传输时，会拷贝整个结构体；高并发百万条消息时，拷贝开销会叠加。
  存指针：通道只拷贝8 字节指针（64 位系统），几乎零开销。

  我目前的InGameMsg结构体中net.Conn(接口)和MsgData(切片)这两个字段都是引用类型，传值不会拷贝底层数据，本身就没有深拷贝成本；
  此时用指针，只是轻量化传递，完全没有副作用。

  值通道一般用在结构体很小、需要高并发安全、不需要修改原数据(只读)的场景下(值拷贝 = 协程间不共享内存，天然无竞争)。

  随后，我会修改为指针chan，采纳该改善。

- 内存对齐机制是指，
	Go 编译器自动执行的「内存排列规则」
	底层原理：64 位 CPU 读取内存，不是 1 个字节 1 个字节读，而是一次性读 8 个字节的块；
	规则：编译器会强制结构体的每个字段，必须放在「对齐的内存地址」上（比如 8 的倍数地址）；
	代价：字段之间会空出一些没用的字节（叫填充字节），浪费一点点空间；
	目的：空间换时间，让 CPU 读取数据更快，不卡顿。

	内存对齐是自动的。
- 切片的存储：
  切片 → 固定占 24 字节（存三个值：数据指针 + 长度 + 容量）
  切片内容多大，不影响其本身的占用。


直到现在，我完成了客户端发送移动指令->服务端计算并存储->服务端全量广播坐标，客户端渲染的逻辑，这比之前的客户端发坐标，服务端更新并广播更加进步。

目前测试下，在541球的压测中未出现卡顿、不同步的情况。

一个已知问题是，ID目前没有重用机制，会持续递增，后续可以修改。
26.04.13
# Python 客户端集成

本文档展示如何在 Python 应用中集成 RAG MySQL 查询系统，包括同步和异步客户端的实现。

## 安装依赖

```bash
pip install websockets asyncio aiohttp requests python-jwt
```

## 异步客户端实现

### 1. 基础异步客户端

```python
import asyncio
import json
import logging
import time
from typing import Dict, List, Optional, Any, Callable
import websockets
from websockets.exceptions import ConnectionClosed, WebSocketException

class RAGAsyncClient:
    """RAG MySQL 查询系统异步客户端"""

    def __init__(self, url: str, **options):
        self.url = url
        self.options = {
            'reconnect_interval': 5,
            'max_reconnect_attempts': 5,
            'timeout': 30,
            **options
        }

        self.websocket = None
        self.session_id = None
        self.message_id = 1
        self.pending_requests = {}
        self.reconnect_attempts = 0
        self.is_authenticated = False

        # 事件回调
        self.event_handlers = {
            'connected': [],
            'disconnected': [],
            'authenticated': [],
            'error': [],
            'message': []
        }

        # 日志配置
        self.logger = logging.getLogger(__name__)

    async def connect(self) -> bool:
        """连接到 RAG 服务器"""
        try:
            self.websocket = await websockets.connect(
                self.url,
                timeout=self.options['timeout']
            )

            self.logger.info("WebSocket 连接已建立")
            self.reconnect_attempts = 0
            await self._emit_event('connected')

            # 启动消息监听
            asyncio.create_task(self._message_listener())

            # 协议协商
            await self._negotiate_protocol()
            return True

        except Exception as e:
            self.logger.error(f"连接失败: {e}")
            await self._emit_event('error', e)
            return False

    async def _negotiate_protocol(self) -> Dict[str, Any]:
        """协议协商"""
        response = await self._send_request('protocol.negotiate', {
            'versions': ['1.0.0']
        })

        if response.get('result'):
            self.logger.info(f"协议协商成功: {response['result']['version']}")
            return response['result']
        else:
            raise Exception(f"协议协商失败: {response.get('error', {}).get('message', '未知错误')}")

    async def authenticate(self, token: str) -> Dict[str, Any]:
        """用户认证"""
        response = await self._send_request('auth.authenticate', {
            'token': token
        })

        if response.get('result', {}).get('success'):
            self.is_authenticated = True
            await self._emit_event('authenticated', response['result']['user'])
            self.logger.info("用户认证成功")
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '认证失败')
            raise Exception(f"认证失败: {error_msg}")

    async def create_session(self, user_id: str) -> Dict[str, Any]:
        """创建会话"""
        response = await self._send_request('session.create', {
            'user_id': user_id
        })

        if response.get('result'):
            self.session_id = response['result']['session_id']
            self.logger.info(f"会话创建成功: {self.session_id}")
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '会话创建失败')
            raise Exception(f"会话创建失败: {error_msg}")

    async def query(self, query_text: str, options: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """执行查询"""
        if not self.session_id:
            raise Exception("会话未建立，请先创建会话")

        query_options = {
            'limit': 20,
            'explain': False,
            'optimize': True,
            **(options or {})
        }

        response = await self._send_request('query.execute', {
            'query': query_text,
            'session_id': self.session_id,
            'options': query_options
        })

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '查询失败')
            raise Exception(f"查询失败: {error_msg}")

    async def explain_query(self, query_text: str) -> Dict[str, Any]:
        """查询解释"""
        response = await self._send_request('query.explain', {
            'query': query_text
        })

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '查询解释失败')
            raise Exception(f"查询解释失败: {error_msg}")

    async def get_suggestions(self, partial: str, limit: int = 5) -> List[str]:
        """获取查询建议"""
        response = await self._send_request('query.suggest', {
            'partial': partial,
            'limit': limit
        })

        if response.get('result'):
            return response['result']['suggestions']
        else:
            error_msg = response.get('error', {}).get('message', '获取建议失败')
            raise Exception(f"获取建议失败: {error_msg}")

    async def get_schema_info(self, table_name: Optional[str] = None,
                            include_relationships: bool = True) -> Dict[str, Any]:
        """获取 Schema 信息"""
        params = {'include_relationships': include_relationships}
        if table_name:
            params['table'] = table_name

        response = await self._send_request('schema.info', params)

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '获取 Schema 失败')
            raise Exception(f"获取 Schema 失败: {error_msg}")

    async def search_schema(self, query: str, limit: int = 10) -> Dict[str, Any]:
        """搜索 Schema"""
        response = await self._send_request('schema.search', {
            'query': query,
            'limit': limit
        })

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', 'Schema 搜索失败')
            raise Exception(f"Schema 搜索失败: {error_msg}")

    async def _send_request(self, method: str, params: Dict[str, Any] = None) -> Dict[str, Any]:
        """发送请求"""
        if not self.websocket:
            raise Exception("WebSocket 连接未建立")

        request_id = f"req-{self.message_id}"
        self.message_id += 1

        message = {
            'type': 'request',
            'id': request_id,
            'method': method,
            'params': params or {}
        }

        # 创建 Future 对象等待响应
        future = asyncio.Future()
        self.pending_requests[request_id] = {
            'future': future,
            'timestamp': time.time()
        }

        try:
            await self.websocket.send(json.dumps(message))

            # 等待响应，设置超时
            response = await asyncio.wait_for(
                future,
                timeout=self.options['timeout']
            )
            return response

        except asyncio.TimeoutError:
            self.pending_requests.pop(request_id, None)
            raise Exception("请求超时")
        except Exception as e:
            self.pending_requests.pop(request_id, None)
            raise e

    async def _message_listener(self):
        """消息监听器"""
        try:
            async for message in self.websocket:
                try:
                    data = json.loads(message)
                    await self._handle_message(data)
                except json.JSONDecodeError as e:
                    self.logger.error(f"消息解析失败: {e}")
                except Exception as e:
                    self.logger.error(f"消息处理失败: {e}")

        except ConnectionClosed:
            self.logger.info("WebSocket 连接已关闭")
            await self._emit_event('disconnected')
            await self._handle_reconnect()
        except Exception as e:
            self.logger.error(f"消息监听器错误: {e}")
            await self._emit_event('error', e)

    async def _handle_message(self, message: Dict[str, Any]):
        """处理接收到的消息"""
        await self._emit_event('message', message)

        if message.get('type') == 'response' and message.get('id'):
            # 处理响应消息
            request_id = message['id']
            if request_id in self.pending_requests:
                future = self.pending_requests[request_id]['future']
                if not future.done():
                    future.set_result(message)
                del self.pending_requests[request_id]

        elif message.get('type') == 'notification':
            # 处理通知消息
            await self._handle_notification(message)

    async def _handle_notification(self, message: Dict[str, Any]):
        """处理通知消息"""
        method = message.get('method')
        params = message.get('params', {})

        if method == 'query.progress':
            await self._emit_event('query_progress', params)
        elif method == 'session.expired':
            self.session_id = None
            await self._emit_event('session_expired', params)
        elif method == 'system.status':
            await self._emit_event('system_status', params)

    async def _handle_reconnect(self):
        """处理重连"""
        if self.reconnect_attempts < self.options['max_reconnect_attempts']:
            self.reconnect_attempts += 1
            self.logger.info(f"尝试重连 ({self.reconnect_attempts}/{self.options['max_reconnect_attempts']})")

            await asyncio.sleep(self.options['reconnect_interval'])

            try:
                await self.connect()
            except Exception as e:
                self.logger.error(f"重连失败: {e}")
        else:
            self.logger.error("达到最大重连次数，停止重连")

    def on(self, event: str, handler: Callable):
        """注册事件处理器"""
        if event in self.event_handlers:
            self.event_handlers[event].append(handler)

    def off(self, event: str, handler: Callable):
        """移除事件处理器"""
        if event in self.event_handlers and handler in self.event_handlers[event]:
            self.event_handlers[event].remove(handler)

    async def _emit_event(self, event: str, data: Any = None):
        """触发事件"""
        if event in self.event_handlers:
            for handler in self.event_handlers[event]:
                try:
                    if asyncio.iscoroutinefunction(handler):
                        await handler(data)
                    else:
                        handler(data)
                except Exception as e:
                    self.logger.error(f"事件处理器错误: {e}")

    async def disconnect(self):
        """断开连接"""
        if self.websocket:
            await self.websocket.close()
            self.websocket = None

        self.session_id = None
        self.is_authenticated = False
        self.pending_requests.clear()

    def is_connected(self) -> bool:
        """检查连接状态"""
        return self.websocket is not None and not self.websocket.closed
```

### 2. 使用示例

```python
import asyncio
import logging

# 配置日志
logging.basicConfig(level=logging.INFO)

async def main():
    # 创建客户端
    client = RAGAsyncClient('ws://localhost:8080/mcp',
                           reconnect_interval=3,
                           max_reconnect_attempts=3)

    # 设置事件处理器
    client.on('connected', lambda: print("客户端已连接"))
    client.on('authenticated', lambda user: print(f"用户已认证: {user}"))
    client.on('error', lambda error: print(f"客户端错误: {error}"))

    try:
        # 连接到服务器
        await client.connect()

        # 用户认证
        await client.authenticate('your-jwt-token')

        # 创建会话
        await client.create_session('python-user')

        # 执行查询
        result = await client.query('查询所有用户信息', {
            'limit': 10,
            'explain': True
        })

        print("查询结果:")
        print(f"SQL: {result.get('sql')}")
        print(f"数据: {result.get('data')}")
        print(f"解释: {result.get('explanation')}")

        # 获取查询建议
        suggestions = await client.get_suggestions('查询用户')
        print(f"查询建议: {suggestions}")

        # 获取 Schema 信息
        schema = await client.get_schema_info('users')
        print(f"Schema 信息: {schema}")

    except Exception as e:
        print(f"操作失败: {e}")
    finally:
        await client.disconnect()

# 运行示例
if __name__ == "__main__":
    asyncio.run(main())
```

## 同步客户端实现

### 1. 基础同步客户端

```python
import json
import threading
import time
import queue
from typing import Dict, List, Optional, Any
import websocket
import logging

class RAGSyncClient:
    """RAG MySQL 查询系统同步客户端"""

    def __init__(self, url: str, **options):
        self.url = url
        self.options = {
            'timeout': 30,
            'ping_interval': 30,
            'ping_timeout': 10,
            **options
        }

        self.ws = None
        self.session_id = None
        self.message_id = 1
        self.pending_requests = {}
        self.is_authenticated = False
        self.is_connected = False

        # 线程安全的队列用于消息处理
        self.message_queue = queue.Queue()
        self.response_events = {}

        # 日志配置
        self.logger = logging.getLogger(__name__)

    def connect(self) -> bool:
        """连接到 RAG 服务器"""
        try:
            websocket.enableTrace(False)
            self.ws = websocket.WebSocketApp(
                self.url,
                on_open=self._on_open,
                on_message=self._on_message,
                on_error=self._on_error,
                on_close=self._on_close
            )

            # 启动 WebSocket 连接（在后台线程中）
            self.ws_thread = threading.Thread(
                target=self.ws.run_forever,
                kwargs={
                    'ping_interval': self.options['ping_interval'],
                    'ping_timeout': self.options['ping_timeout']
                }
            )
            self.ws_thread.daemon = True
            self.ws_thread.start()

            # 等待连接建立
            start_time = time.time()
            while not self.is_connected and time.time() - start_time < self.options['timeout']:
                time.sleep(0.1)

            if self.is_connected:
                # 协议协商
                self._negotiate_protocol()
                return True
            else:
                raise Exception("连接超时")

        except Exception as e:
            self.logger.error(f"连接失败: {e}")
            return False

    def _negotiate_protocol(self) -> Dict[str, Any]:
        """协议协商"""
        response = self._send_request('protocol.negotiate', {
            'versions': ['1.0.0']
        })

        if response.get('result'):
            self.logger.info(f"协议协商成功: {response['result']['version']}")
            return response['result']
        else:
            raise Exception(f"协议协商失败: {response.get('error', {}).get('message', '未知错误')}")

    def authenticate(self, token: str) -> Dict[str, Any]:
        """用户认证"""
        response = self._send_request('auth.authenticate', {
            'token': token
        })

        if response.get('result', {}).get('success'):
            self.is_authenticated = True
            self.logger.info("用户认证成功")
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '认证失败')
            raise Exception(f"认证失败: {error_msg}")

    def create_session(self, user_id: str) -> Dict[str, Any]:
        """创建会话"""
        response = self._send_request('session.create', {
            'user_id': user_id
        })

        if response.get('result'):
            self.session_id = response['result']['session_id']
            self.logger.info(f"会话创建成功: {self.session_id}")
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '会话创建失败')
            raise Exception(f"会话创建失败: {error_msg}")

    def query(self, query_text: str, options: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """执行查询"""
        if not self.session_id:
            raise Exception("会话未建立，请先创建会话")

        query_options = {
            'limit': 20,
            'explain': False,
            'optimize': True,
            **(options or {})
        }

        response = self._send_request('query.execute', {
            'query': query_text,
            'session_id': self.session_id,
            'options': query_options
        })

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '查询失败')
            raise Exception(f"查询失败: {error_msg}")

    def explain_query(self, query_text: str) -> Dict[str, Any]:
        """查询解释"""
        response = self._send_request('query.explain', {
            'query': query_text
        })

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '查询解释失败')
            raise Exception(f"查询解释失败: {error_msg}")

    def get_suggestions(self, partial: str, limit: int = 5) -> List[str]:
        """获取查询建议"""
        response = self._send_request('query.suggest', {
            'partial': partial,
            'limit': limit
        })

        if response.get('result'):
            return response['result']['suggestions']
        else:
            error_msg = response.get('error', {}).get('message', '获取建议失败')
            raise Exception(f"获取建议失败: {error_msg}")

    def get_schema_info(self, table_name: Optional[str] = None,
                       include_relationships: bool = True) -> Dict[str, Any]:
        """获取 Schema 信息"""
        params = {'include_relationships': include_relationships}
        if table_name:
            params['table'] = table_name

        response = self._send_request('schema.info', params)

        if response.get('result'):
            return response['result']
        else:
            error_msg = response.get('error', {}).get('message', '获取 Schema 失败')
            raise Exception(f"获取 Schema 失败: {error_msg}")

    def _send_request(self, method: str, params: Dict[str, Any] = None) -> Dict[str, Any]:
        """发送请求"""
        if not self.is_connected:
            raise Exception("WebSocket 连接未建立")

        request_id = f"req-{self.message_id}"
        self.message_id += 1

        message = {
            'type': 'request',
            'id': request_id,
            'method': method,
            'params': params or {}
        }

        # 创建事件等待响应
        response_event = threading.Event()
        self.response_events[request_id] = {
            'event': response_event,
            'response': None,
            'timestamp': time.time()
        }

        try:
            # 发送消息
            self.ws.send(json.dumps(message))

            # 等待响应
            if response_event.wait(timeout=self.options['timeout']):
                response = self.response_events[request_id]['response']
                del self.response_events[request_id]
                return response
            else:
                del self.response_events[request_id]
                raise Exception("请求超时")

        except Exception as e:
            self.response_events.pop(request_id, None)
            raise e

    def _on_open(self, ws):
        """连接打开回调"""
        self.is_connected = True
        self.logger.info("WebSocket 连接已建立")

    def _on_message(self, ws, message):
        """消息接收回调"""
        try:
            data = json.loads(message)
            self._handle_message(data)
        except json.JSONDecodeError as e:
            self.logger.error(f"消息解析失败: {e}")
        except Exception as e:
            self.logger.error(f"消息处理失败: {e}")

    def _on_error(self, ws, error):
        """错误回调"""
        self.logger.error(f"WebSocket 错误: {error}")

    def _on_close(self, ws, close_status_code, close_msg):
        """连接关闭回调"""
        self.is_connected = False
        self.logger.info("WebSocket 连接已关闭")

    def _handle_message(self, message: Dict[str, Any]):
        """处理接收到的消息"""
        if message.get('type') == 'response' and message.get('id'):
            # 处理响应消息
            request_id = message['id']
            if request_id in self.response_events:
                self.response_events[request_id]['response'] = message
                self.response_events[request_id]['event'].set()

        elif message.get('type') == 'notification':
            # 处理通知消息
            self._handle_notification(message)

    def _handle_notification(self, message: Dict[str, Any]):
        """处理通知消息"""
        method = message.get('method')
        params = message.get('params', {})

        if method == 'session.expired':
            self.session_id = None
            self.logger.warning("会话已过期")

    def disconnect(self):
        """断开连接"""
        if self.ws:
            self.ws.close()
            self.ws = None

        self.is_connected = False
        self.session_id = None
        self.is_authenticated = False
        self.response_events.clear()
```

### 2. 同步客户端使用示例

```python
import logging

# 配置日志
logging.basicConfig(level=logging.INFO)

def main():
    # 创建同步客户端
    client = RAGSyncClient('ws://localhost:8080/mcp', timeout=30)

    try:
        # 连接到服务器
        if not client.connect():
            print("连接失败")
            return

        # 用户认证
        client.authenticate('your-jwt-token')
        print("用户认证成功")

        # 创建会话
        client.create_session('python-sync-user')
        print("会话创建成功")

        # 执行查询
        result = client.query('查询所有用户信息', {
            'limit': 10,
            'explain': True
        })

        print("查询结果:")
        print(f"SQL: {result.get('sql')}")
        print(f"数据条数: {len(result.get('data', []))}")
        print(f"解释: {result.get('explanation')}")

        # 获取查询建议
        suggestions = client.get_suggestions('查询用户')
        print(f"查询建议: {suggestions}")

        # 获取 Schema 信息
        schema = client.get_schema_info('users')
        print(f"用户表字段数: {len(schema.get('table', {}).get('columns', []))}")

    except Exception as e:
        print(f"操作失败: {e}")
    finally:
        client.disconnect()
        print("连接已断开")

if __name__ == "__main__":
    main()
```

## Flask 集成示例

### 1. Flask 应用集成

```python
from flask import Flask, request, jsonify
import threading
import time

app = Flask(__name__)

# 全局客户端实例
rag_client = None
client_lock = threading.Lock()

def get_rag_client():
    """获取 RAG 客户端实例（线程安全）"""
    global rag_client

    with client_lock:
        if rag_client is None or not rag_client.is_connected:
            rag_client = RAGSyncClient('ws://localhost:8080/mcp')
            if rag_client.connect():
                rag_client.authenticate('server-jwt-token')
                rag_client.create_session('flask-server')
            else:
                raise Exception("无法连接到 RAG 服务器")

        return rag_client

@app.route('/api/query', methods=['POST'])
def query_endpoint():
    """查询接口"""
    try:
        data = request.get_json()
        query_text = data.get('query')
        options = data.get('options', {})

        if not query_text:
            return jsonify({'error': '查询内容不能为空'}), 400

        client = get_rag_client()
        result = client.query(query_text, options)

        return jsonify(result)

    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/schema/<table_name>', methods=['GET'])
def schema_endpoint(table_name=None):
    """Schema 信息接口"""
    try:
        client = get_rag_client()
        schema = client.get_schema_info(table_name)

        return jsonify(schema)

    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/suggestions', methods=['GET'])
def suggestions_endpoint():
    """查询建议接口"""
    try:
        partial = request.args.get('q', '')
        limit = int(request.args.get('limit', 5))

        if not partial:
            return jsonify({'error': '查询参数不能为空'}), 400

        client = get_rag_client()
        suggestions = client.get_suggestions(partial, limit)

        return jsonify({'suggestions': suggestions})

    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/explain', methods=['POST'])
def explain_endpoint():
    """查询解释接口"""
    try:
        data = request.get_json()
        query_text = data.get('query')

        if not query_text:
            return jsonify({'error': '查询内容不能为空'}), 400

        client = get_rag_client()
        explanation = client.explain_query(query_text)

        return jsonify(explanation)

    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查"""
    try:
        client = get_rag_client()
        return jsonify({
            'status': 'healthy',
            'connected': client.is_connected,
            'authenticated': client.is_authenticated,
            'session_id': client.session_id
        })
    except Exception as e:
        return jsonify({
            'status': 'unhealthy',
            'error': str(e)
        }), 500

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
```

## Django 集成示例

### 1. Django 视图集成

```python
# views.py
from django.http import JsonResponse
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods
import json
import threading

# 全局客户端实例
_rag_client = None
_client_lock = threading.Lock()

def get_rag_client():
    """获取 RAG 客户端实例"""
    global _rag_client

    with _client_lock:
        if _rag_client is None or not _rag_client.is_connected:
            _rag_client = RAGSyncClient('ws://localhost:8080/mcp')
            if _rag_client.connect():
                _rag_client.authenticate('django-jwt-token')
                _rag_client.create_session('django-server')
            else:
                raise Exception("无法连接到 RAG 服务器")

        return _rag_client

@csrf_exempt
@require_http_methods(["POST"])
def query_view(request):
    """查询视图"""
    try:
        data = json.loads(request.body)
        query_text = data.get('query')
        options = data.get('options', {})

        if not query_text:
            return JsonResponse({'error': '查询内容不能为空'}, status=400)

        client = get_rag_client()
        result = client.query(query_text, options)

        return JsonResponse(result)

    except Exception as e:
        return JsonResponse({'error': str(e)}, status=500)

@require_http_methods(["GET"])
def schema_view(request, table_name=None):
    """Schema 视图"""
    try:
        client = get_rag_client()
        schema = client.get_schema_info(table_name)

        return JsonResponse(schema)

    except Exception as e:
        return JsonResponse({'error': str(e)}, status=500)

@require_http_methods(["GET"])
def suggestions_view(request):
    """建议视图"""
    try:
        partial = request.GET.get('q', '')
        limit = int(request.GET.get('limit', 5))

        if not partial:
            return JsonResponse({'error': '查询参数不能为空'}, status=400)

        client = get_rag_client()
        suggestions = client.get_suggestions(partial, limit)

        return JsonResponse({'suggestions': suggestions})

    except Exception as e:
        return JsonResponse({'error': str(e)}, status=500)
```

### 2. Django URL 配置

```python
# urls.py
from django.urls import path
from . import views

urlpatterns = [
    path('api/query/', views.query_view, name='query'),
    path('api/schema/', views.schema_view, name='schema'),
    path('api/schema/<str:table_name>/', views.schema_view, name='schema_table'),
    path('api/suggestions/', views.suggestions_view, name='suggestions'),
]
```

## 错误处理和重试

### 1. 重试装饰器

```python
import functools
import time
import random

def retry_on_failure(max_retries=3, delay=1, backoff=2, jitter=True):
    """重试装饰器"""
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            retries = 0
            while retries < max_retries:
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    retries += 1
                    if retries >= max_retries:
                        raise e

                    # 计算延迟时间
                    wait_time = delay * (backoff ** (retries - 1))
                    if jitter:
                        wait_time += random.uniform(0, wait_time * 0.1)

                    print(f"操作失败，{wait_time:.2f}秒后重试 ({retries}/{max_retries}): {e}")
                    time.sleep(wait_time)

            return None
        return wrapper
    return decorator

# 使用示例
class RetryableRAGClient(RAGSyncClient):
    @retry_on_failure(max_retries=3, delay=1, backoff=2)
    def query_with_retry(self, query_text: str, options: Optional[Dict[str, Any]] = None):
        return self.query(query_text, options)

    @retry_on_failure(max_retries=2, delay=0.5)
    def get_suggestions_with_retry(self, partial: str, limit: int = 5):
        return self.get_suggestions(partial, limit)
```

### 2. 连接池管理

```python
import queue
import threading
from contextlib import contextmanager

class RAGConnectionPool:
    """RAG 客户端连接池"""

    def __init__(self, url: str, pool_size: int = 5, **options):
        self.url = url
        self.pool_size = pool_size
        self.options = options

        self.pool = queue.Queue(maxsize=pool_size)
        self.lock = threading.Lock()

        # 初始化连接池
        self._initialize_pool()

    def _initialize_pool(self):
        """初始化连接池"""
        for _ in range(self.pool_size):
            client = RAGSyncClient(self.url, **self.options)
            if client.connect():
                client.authenticate('pool-jwt-token')
                client.create_session(f'pool-session-{threading.current_thread().ident}')
                self.pool.put(client)
            else:
                raise Exception("无法初始化连接池")

    @contextmanager
    def get_client(self, timeout: int = 30):
        """获取客户端连接（上下文管理器）"""
        client = None
        try:
            client = self.pool.get(timeout=timeout)

            # 检查连接状态
            if not client.is_connected:
                client.connect()
                client.authenticate('pool-jwt-token')
                client.create_session(f'pool-session-{threading.current_thread().ident}')

            yield client

        except queue.Empty:
            raise Exception("连接池已满，无法获取连接")
        finally:
            if client:
                self.pool.put(client)

    def execute_query(self, query_text: str, options: Optional[Dict[str, Any]] = None):
        """使用连接池执行查询"""
        with self.get_client() as client:
            return client.query(query_text, options)

    def close_all(self):
        """关闭所有连接"""
        while not self.pool.empty():
            try:
                client = self.pool.get_nowait()
                client.disconnect()
            except queue.Empty:
                break

# 使用示例
pool = RAGConnectionPool('ws://localhost:8080/mcp', pool_size=3)

try:
    result = pool.execute_query('查询用户信息')
    print(result)
finally:
    pool.close_all()
```

## 测试示例

### 1. 单元测试

```python
import unittest
from unittest.mock import Mock, patch
import json

class TestRAGSyncClient(unittest.TestCase):

    def setUp(self):
        self.client = RAGSyncClient('ws://localhost:8080/mcp')

    def tearDown(self):
        if self.client.is_connected:
            self.client.disconnect()

    @patch('websocket.WebSocketApp')
    def test_connect(self, mock_ws):
        """测试连接功能"""
        # 模拟 WebSocket 连接
        mock_ws_instance = Mock()
        mock_ws.return_value = mock_ws_instance

        # 模拟连接成功
        self.client.is_connected = True

        result = self.client.connect()
        self.assertTrue(result)
        self.assertTrue(self.client.is_connected)

    def test_query_without_session(self):
        """测试没有会话时的查询"""
        with self.assertRaises(Exception) as context:
            self.client.query('test query')

        self.assertIn('会话未建立', str(context.exception))

    @patch.object(RAGSyncClient, '_send_request')
    def test_authenticate(self, mock_send):
        """测试认证功能"""
        # 模拟成功响应
        mock_send.return_value = {
            'result': {
                'success': True,
                'user': {'id': 1, 'name': 'test'}
            }
        }

        result = self.client.authenticate('test-token')

        self.assertTrue(self.client.is_authenticated)
        self.assertEqual(result['user']['name'], 'test')
        mock_send.assert_called_once_with('auth.authenticate', {'token': 'test-token'})

class TestRAGAsyncClient(unittest.TestCase):

    def setUp(self):
        self.client = RAGAsyncClient('ws://localhost:8080/mcp')

    async def test_connect(self):
        """测试异步连接"""
        with patch('websockets.connect') as mock_connect:
            mock_ws = Mock()
            mock_connect.return_value = mock_ws

            # 模拟协议协商
            with patch.object(self.client, '_negotiate_protocol') as mock_negotiate:
                mock_negotiate.return_value = {'version': '1.0.0'}

                result = await self.client.connect()
                self.assertTrue(result)

if __name__ == '__main__':
    unittest.main()
```

### 2. 集成测试

```python
import asyncio
import pytest

@pytest.mark.asyncio
async def test_full_workflow():
    """测试完整工作流程"""
    client = RAGAsyncClient('ws://localhost:8080/mcp')

    try:
        # 连接
        await client.connect()
        assert client.is_connected()

        # 认证
        await client.authenticate('test-token')
        assert client.is_authenticated

        # 创建会话
        await client.create_session('test-user')
        assert client.session_id is not None

        # 执行查询
        result = await client.query('查询用户信息', {'limit': 5})
        assert result['success'] is True
        assert 'data' in result

        # 获取建议
        suggestions = await client.get_suggestions('查询')
        assert isinstance(suggestions, list)

    finally:
        await client.disconnect()

def test_sync_workflow():
    """测试同步工作流程"""
    client = RAGSyncClient('ws://localhost:8080/mcp')

    try:
        # 连接
        assert client.connect() is True

        # 认证
        client.authenticate('test-token')
        assert client.is_authenticated is True

        # 创建会话
        client.create_session('test-user')
        assert client.session_id is not None

        # 执行查询
        result = client.query('查询用户信息', {'limit': 5})
        assert result['success'] is True

    finally:
        client.disconnect()
```

## 性能优化

### 1. 批量查询

```python
class BatchRAGClient(RAGSyncClient):
    """支持批量查询的客户端"""

    def batch_query(self, queries: List[Dict[str, Any]], max_workers: int = 5) -> List[Dict[str, Any]]:
        """批量执行查询"""
        import concurrent.futures

        results = []

        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            # 提交所有查询任务
            future_to_query = {
                executor.submit(self.query, q['query'], q.get('options', {})): q
                for q in queries
            }

            # 收集结果
            for future in concurrent.futures.as_completed(future_to_query):
                query_info = future_to_query[future]
                try:
                    result = future.result()
                    results.append({
                        'query': query_info['query'],
                        'result': result,
                        'success': True
                    })
                except Exception as e:
                    results.append({
                        'query': query_info['query'],
                        'error': str(e),
                        'success': False
                    })

        return results

# 使用示例
client = BatchRAGClient('ws://localhost:8080/mcp')
client.connect()
client.authenticate('token')
client.create_session('batch-user')

queries = [
    {'query': '查询用户总数'},
    {'query': '查询订单总数'},
    {'query': '查询产品总数'},
]

results = client.batch_query(queries)
for result in results:
    print(f"查询: {result['query']}")
    if result['success']:
        print(f"结果: {result['result']}")
    else:
        print(f"错误: {result['error']}")
```

### 2. 缓存机制

```python
import hashlib
import pickle
import time
from typing import Optional

class CachedRAGClient(RAGSyncClient):
    """带缓存的 RAG 客户端"""

    def __init__(self, url: str, cache_ttl: int = 300, **options):
        super().__init__(url, **options)
        self.cache = {}
        self.cache_ttl = cache_ttl

    def _get_cache_key(self, query_text: str, options: Dict[str, Any]) -> str:
        """生成缓存键"""
        cache_data = f"{query_text}:{json.dumps(options, sort_keys=True)}"
        return hashlib.md5(cache_data.encode()).hexdigest()

    def query_with_cache(self, query_text: str, options: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """带缓存的查询"""
        options = options or {}
        cache_key = self._get_cache_key(query_text, options)

        # 检查缓存
        if cache_key in self.cache:
            cached_data = self.cache[cache_key]
            if time.time() - cached_data['timestamp'] < self.cache_ttl:
                print(f"从缓存返回结果: {query_text}")
                return cached_data['result']
            else:
                # 缓存过期，删除
                del self.cache[cache_key]

        # 执行查询
        result = self.query(query_text, options)

        # 缓存结果
        if result.get('success'):
            self.cache[cache_key] = {
                'result': result,
                'timestamp': time.time()
            }

        return result

    def clear_cache(self):
        """清空缓存"""
        self.cache.clear()

    def get_cache_stats(self) -> Dict[str, Any]:
        """获取缓存统计"""
        total_entries = len(self.cache)
        expired_entries = sum(
            1 for data in self.cache.values()
            if time.time() - data['timestamp'] >= self.cache_ttl
        )

        return {
            'total_entries': total_entries,
            'valid_entries': total_entries - expired_entries,
            'expired_entries': expired_entries
        }
```

这个 Python 客户端集成指南提供了完整的同步和异步客户端实现，包括 Flask 和 Django 集成示例、错误处理、性能优化和测试用例。开发者可以根据自己的需求选择合适的集成方式。

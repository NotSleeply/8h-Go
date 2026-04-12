import { useEffect, useMemo, useRef, useState } from "react";

type ChatLine = {
  id: string;
  text: string;
  mine?: boolean;
};

const sessionsSeed = ["system", "alice", "bob", "group:team"];

export function App() {
  const [nickname, setNickname] = useState("");
  const [connected, setConnected] = useState(false);
  const [sessions] = useState(sessionsSeed);
  const [active, setActive] = useState("system");
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Record<string, ChatLine[]>>({
    system: [{ id: "s1", text: "欢迎使用 8h-GoIM Web Client。" }],
  });
  const wsRef = useRef<WebSocket | null>(null);

  const current = useMemo(() => messages[active] ?? [], [messages, active]);

  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

  const append = (channel: string, item: ChatLine) => {
    setMessages((prev) => ({
      ...prev,
      [channel]: [...(prev[channel] ?? []), item],
    }));
  };

  const connect = () => {
    if (connected) return;
    const ws = new WebSocket("ws://127.0.0.1:8080/ws");
    wsRef.current = ws;
    ws.onopen = () => {
      setConnected(true);
      append("system", { id: String(Date.now()), text: "WebSocket 已连接。" });
      if (nickname.trim()) {
        ws.send(JSON.stringify({ type: "rename", name: nickname.trim() }));
      }
    };
    ws.onmessage = (evt) => {
      const text = String(evt.data ?? "");
      append(active, { id: crypto.randomUUID(), text });
    };
    ws.onclose = () => {
      setConnected(false);
      append("system", { id: String(Date.now()), text: "WebSocket 已断开。" });
    };
    ws.onerror = () => {
      append("system", { id: String(Date.now()), text: "连接异常，请检查 Gate 地址。" });
    };
  };

  const send = () => {
    const body = input.trim();
    if (!body) return;
    append(active, { id: crypto.randomUUID(), text: body, mine: true });
    setInput("");

    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      if (active.startsWith("group:")) {
        const gid = active.replace("group:", "");
        ws.send(`gt|${gid}|${body}`);
      } else if (active === "system") {
        ws.send(body);
      } else {
        ws.send(`to|${active}|${body}`);
      }
    } else {
      append("system", { id: crypto.randomUUID(), text: "未连接 Gate，消息仅本地展示。" });
    }
  };

  return (
    <div className="page">
      <header className="topbar">
        <div>
          <h1>8h-GoIM</h1>
          <p>React + TypeScript + Vite Demo Client</p>
        </div>
        <div className="login">
          <input
            value={nickname}
            placeholder="输入昵称"
            onChange={(e) => setNickname(e.target.value)}
          />
          <button onClick={connect} disabled={connected}>
            {connected ? "已连接" : "连接 Gate"}
          </button>
        </div>
      </header>

      <main className="layout">
        <aside className="sidebar">
          <h2>会话</h2>
          {sessions.map((s) => (
            <button
              key={s}
              className={s === active ? "session active" : "session"}
              onClick={() => setActive(s)}
            >
              {s}
            </button>
          ))}
        </aside>

        <section className="chat">
          <div className="chat-head">{active}</div>
          <div className="messages">
            {current.map((m) => (
              <div key={m.id} className={m.mine ? "msg mine" : "msg"}>
                {m.text}
              </div>
            ))}
          </div>
          <div className="composer">
            <input
              value={input}
              placeholder="输入消息..."
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && send()}
            />
            <button onClick={send}>发送</button>
          </div>
        </section>
      </main>
    </div>
  );
}

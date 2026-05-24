import { Conversation, KnowledgeBase } from "../../App";

interface WelcomeHomeProps {
  brandName?: string;
  conversations: Conversation[];
  activeConversationId: string | null;
  knowledgeBases: KnowledgeBase[];
  selectedKnowledgeBaseId: string | null;
  hasKnowledgeBase: boolean;
  onSelectConversation: (conversationId: string) => void;
  onSelectKnowledgeBase: (knowledgeBaseId: string) => void;
  onSendPrompt: (text: string) => void;
}

// 中度密度欢迎区：2 列（最近会话 / 推荐 prompts）+ 下方知识库快捷入口行。
// 由 ChatArea 在 activeConversation.messages.length === 0 时渲染。
// 纯文字、纯色彩、纯留白——无 emoji 装饰。

const KB_PROMPTS = [
  "请总结当前知识库的核心观点",
  "请列出这个知识库中最关键的结论",
  "如果基于当前资料开始实现，下一步建议是什么？",
  "为我归纳并对比知识库中的不同观点",
];

const GENERAL_PROMPTS = [
  "你好，介绍一下你自己",
  "用通俗的语言解释一个技术概念",
  "帮我列一份待办清单的思路",
  "推荐一个学习新技术的高效方法",
];

const formatRecentTime = (value: string) => {
  const date = new Date(value);
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  if (sameDay) {
    return date.toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
    });
  }
  return date.toLocaleDateString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
  });
};

export default function WelcomeHome({
  brandName = "知港 KnowPort",
  conversations,
  activeConversationId,
  knowledgeBases,
  selectedKnowledgeBaseId,
  hasKnowledgeBase,
  onSelectConversation,
  onSelectKnowledgeBase,
  onSendPrompt,
}: WelcomeHomeProps) {
  const recent = conversations
    .filter((c) => c.id !== activeConversationId)
    .slice(0, 3);
  const prompts = hasKnowledgeBase ? KB_PROMPTS : GENERAL_PROMPTS;

  return (
    <div className="welcome-home">
      <header className="welcome-home-header">
        <span className="welcome-home-eyebrow">START</span>
        <h2>欢迎回到 {brandName}</h2>
        <p>
          {hasKnowledgeBase
            ? "向当前知识库提问，或从下方的常用问题开始。"
            : "选择一个知识库以开启 RAG 问答，或者直接对话。"}
        </p>
      </header>

      <div className="welcome-home-grid">
        <section
          className="welcome-home-col welcome-home-recent"
          aria-label="最近会话"
        >
          <div className="welcome-home-col-head">
            <h3>最近会话</h3>
            <span className="welcome-home-col-hint">点击继续</span>
          </div>
          {recent.length === 0 ? (
            <div className="welcome-empty">
              还没有其他会话，从右侧的推荐问题开始吧
            </div>
          ) : (
            <ul className="welcome-recent-list">
              {recent.map((conv) => (
                <li key={conv.id}>
                  <button
                    type="button"
                    className="welcome-recent-card"
                    onClick={() => onSelectConversation(conv.id)}
                  >
                    <span className="welcome-recent-title" title={conv.title}>
                      {conv.title || "未命名对话"}
                    </span>
                    <span className="welcome-recent-meta">
                      <span>{conv.messages.length} 条消息</span>
                      <span>·</span>
                      <span>{formatRecentTime(conv.updatedAt)}</span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section
          className="welcome-home-col welcome-home-prompts"
          aria-label="推荐问题"
        >
          <div className="welcome-home-col-head">
            <h3>试试这样问</h3>
            <span className="welcome-home-col-hint">
              {hasKnowledgeBase ? "围绕当前知识库" : "通用对话起点"}
            </span>
          </div>
          <div className="welcome-prompt-grid">
            {prompts.map((text) => (
              <button
                key={text}
                type="button"
                className="welcome-prompt-chip"
                onClick={() => onSendPrompt(text)}
                title="点击发送这个问题"
              >
                {text}
              </button>
            ))}
          </div>
        </section>
      </div>

      {knowledgeBases.length > 0 && (
        <section className="welcome-home-kb-row" aria-label="知识库快捷入口">
          <div className="welcome-home-kb-head">
            <h3>切换知识库</h3>
            <span className="welcome-home-col-hint">
              {selectedKnowledgeBaseId ? "已绑定" : "未绑定"}
            </span>
          </div>
          <div className="welcome-kb-chips">
            <button
              type="button"
              className={`welcome-kb-chip${selectedKnowledgeBaseId ? "" : " welcome-kb-chip--active"}`}
              onClick={() => onSelectKnowledgeBase("")}
            >
              <span className="welcome-kb-chip-name">无知识库</span>
              <span className="welcome-kb-chip-meta">通用对话</span>
            </button>
            {knowledgeBases.map((kb) => {
              const active = kb.id === selectedKnowledgeBaseId;
              const docCount = kb.documents?.length ?? 0;
              return (
                <button
                  key={kb.id}
                  type="button"
                  className={`welcome-kb-chip${active ? " welcome-kb-chip--active" : ""}`}
                  onClick={() => onSelectKnowledgeBase(kb.id)}
                  title={kb.name}
                >
                  <span className="welcome-kb-chip-name">{kb.name}</span>
                  <span className="welcome-kb-chip-meta">
                    {docCount} 份文档
                  </span>
                </button>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

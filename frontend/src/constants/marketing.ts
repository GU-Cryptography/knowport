// 营销文案常量：被 LandingPage / LoginPage 共用，避免内容漂移。
// 注意：刻意去除 icon 字段（用户约束：无替代图标，仅文字+色彩+留白）。

export interface ValuePoint {
  title: string;
  desc: string;
}

export interface Scenario {
  label: string; // 场景关键词（短词）
  detail: string; // 一句话解释
}

// 核心能力（右列）
export const VALUE_POINTS: ReadonlyArray<ValuePoint> = [
  {
    title: "注册即用 · 零门槛体验",
    desc: "注册即用，无需配置环境与模型",
  },
  {
    title: "代码开源 · 可自部署",
    desc: "源码开源，自托管，数据自有可控",
  },
  {
    title: "RAG 智能问答",
    desc: "多知识库 · 多会话 · 段落级引用",
  },
  {
    title: "多模型自由接入",
    desc: "Ollama / OpenAI / DeepSeek 任意切",
  },
];

// 使用场景（左列）—— 与 VALUE_POINTS 形成「场景 ↔ 能力」对比
export const SCENARIOS: ReadonlyArray<Scenario> = [
  {
    label: "个人知识库",
    detail: "书籍、笔记、收藏汇成专属港口",
  },
  {
    label: "文献研读",
    detail: "论文按段引用，告别从头通读",
  },
  {
    label: "团队文档问答",
    detail: "内网部署，员工对着手册提问",
  },
  {
    label: "代码库速读",
    detail: "源码与设计文档随问随答",
  },
];

// 外链（顶部导航）
export const EXTERNAL_LINKS = {
  github: "https://github.com/", // 实际仓库地址由部署方替换；保留占位符避免空跳转
} as const;

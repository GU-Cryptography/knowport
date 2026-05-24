import React, { useEffect, useRef, useState } from "react";
import {
  AppConfig,
  ChatConfig,
  ChatModeSettings,
  EmbeddingConfig,
} from "../../App";
import { apiFetch } from "../../api/auth";

interface ModelComboboxProps {
  value: string;
  onChange: (value: string) => void;
  models: string[];
  placeholder?: string;
}

// Input + ▾ button + absolute-positioned dropdown. Closes on outside click.
// User can still type a custom value (preserves manual entry).
const ModelCombobox: React.FC<ModelComboboxProps> = ({
  value,
  onChange,
  models,
  placeholder,
}) => {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  return (
    <div className="model-combobox" ref={wrapRef}>
      <input
        className="model-combobox-input"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
      <button
        type="button"
        className="model-combobox-toggle"
        onClick={() => setOpen((prev) => !prev)}
        disabled={models.length === 0}
        title={
          models.length === 0 ? "请先点测试连接获取模型列表" : "展开模型列表"
        }
      >
        ▾
      </button>
      {open && models.length > 0 && (
        <ul className="model-combobox-list">
          {models.map((m) => (
            <li key={m}>
              <button
                type="button"
                className={`model-combobox-item ${m === value ? "active" : ""}`}
                onClick={() => {
                  onChange(m);
                  setOpen(false);
                }}
              >
                {m}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

interface SettingsPanelProps {
  config: AppConfig;
  onClose: () => void;
  onChatConfigChange: <K extends keyof ChatConfig>(
    key: K,
    value: ChatConfig[K],
  ) => void;
  onEmbeddingConfigChange: <K extends keyof EmbeddingConfig>(
    key: K,
    value: EmbeddingConfig[K],
  ) => void;
  onSaveSettings: () => Promise<void>;
  chatModeSettings: ChatModeSettings;
  onThinkModelChange: (value: string) => void;
  onCopyMcpToken: () => Promise<void>;
  onResetMcpToken: () => Promise<void>;
}

const SettingsPanel: React.FC<SettingsPanelProps> = ({
  config,
  onClose,
  onChatConfigChange,
  onEmbeddingConfigChange,
  onSaveSettings,
  chatModeSettings,
  onThinkModelChange,
  onCopyMcpToken,
  onResetMcpToken,
}) => {
  const [mcpFeedback, setMcpFeedback] = useState("");
  const [isMcpTokenVisible, setIsMcpTokenVisible] = useState(false);

  type SaveStatus = "idle" | "saving" | "saved" | "error";
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");
  const [saveError, setSaveError] = useState("");

  type ProbeStatus = "idle" | "loading" | "success" | "error";
  interface ProbeState {
    status: ProbeStatus;
    models: string[];
    protocol: string;
    cached: boolean;
    error: string;
  }
  const initialProbe: ProbeState = {
    status: "idle",
    models: [],
    protocol: "",
    cached: false,
    error: "",
  };
  const [chatProbe, setChatProbe] = useState<ProbeState>(initialProbe);
  const [embeddingProbe, setEmbeddingProbe] =
    useState<ProbeState>(initialProbe);

  const runProbe = async (
    kind: "chat" | "embedding",
    apiUrl: string,
    apiKey: string,
    setState: React.Dispatch<React.SetStateAction<ProbeState>>,
  ) => {
    if (!apiUrl.trim()) {
      setState({
        ...initialProbe,
        status: "error",
        error: "请先填写 Base URL",
      });
      return;
    }
    setState({ ...initialProbe, status: "loading" });
    try {
      const response = await apiFetch("/api/config/models/probe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          api_url: apiUrl.trim(),
          api_key: apiKey.trim(),
          kind,
        }),
      });
      const data = await response.json();
      if (!response.ok) {
        setState({
          ...initialProbe,
          status: "error",
          error: data?.error?.message || `探测失败 (${response.status})`,
        });
        return;
      }
      setState({
        status: "success",
        models: Array.isArray(data.models) ? data.models : [],
        protocol: data.protocol || "",
        cached: Boolean(data.cached),
        error: "",
      });
    } catch (err) {
      setState({
        ...initialProbe,
        status: "error",
        error: err instanceof Error ? err.message : "探测异常",
      });
    }
  };

  const handleProbeChat = () =>
    runProbe("chat", config.chat.baseUrl, config.chat.apiKey, setChatProbe);
  const handleProbeEmbedding = () =>
    runProbe(
      "embedding",
      config.embedding.baseUrl,
      config.embedding.apiKey,
      setEmbeddingProbe,
    );

  const renderProbeFeedback = (probe: ProbeState) => {
    if (probe.status === "loading") {
      return <small className="settings-feedback">探测中...</small>;
    }
    if (probe.status === "error") {
      return (
        <small className="settings-feedback settings-feedback-error">
          {probe.error}
        </small>
      );
    }
    if (probe.status === "success") {
      return (
        <small className="settings-feedback settings-feedback-success">
          发现 {probe.models.length} 个模型 ({probe.protocol}
          {probe.cached ? " · 缓存" : ""})
        </small>
      );
    }
    return null;
  };

  const handleCopyToken = async () => {
    try {
      await onCopyMcpToken();
      setMcpFeedback("Token 已复制");
    } catch {
      setMcpFeedback("复制失败");
    }
  };

  const handleResetToken = async () => {
    try {
      await onResetMcpToken();
      setMcpFeedback("Token 已重置");
    } catch {
      setMcpFeedback("重置失败");
    }
  };

  // 显式保存：调用父级 onSave 重发后端，给用户明确反馈
  // 自动保存（onChange 触发）仍在工作，这里只是"再确认一次"
  const handleSave = async () => {
    setSaveStatus("saving");
    setSaveError("");
    try {
      await onSaveSettings();
      setSaveStatus("saved");
      window.setTimeout(() => {
        setSaveStatus((current) => (current === "saved" ? "idle" : current));
      }, 2200);
    } catch (err) {
      setSaveStatus("error");
      setSaveError(err instanceof Error ? err.message : "保存失败");
    }
  };
  return (
    <div className="settings-modal-backdrop" onClick={onClose}>
      <div
        className="settings-modal settings-modal-single"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="settings-modal-header">
          <div>
            <h3>AI 设置</h3>
            <p>分别管理聊天模型与 Embedding 模型配置</p>
          </div>
          <button
            type="button"
            className="ghost-btn settings-close-btn"
            onClick={onClose}
          >
            关闭
          </button>
        </div>

        <div className="settings-modal-scroll">
          <section className="settings-panel-block ai-config-panel single-column">
            <div className="section-title-row knowledge-panel-header">
              <h3>聊天模型</h3>
            </div>

            <div className="ai-config-fields">
              <label className="settings-field">
                <span>Provider</span>
                <select
                  value={config.chat.provider}
                  onChange={(event) =>
                    onChatConfigChange(
                      "provider",
                      event.target.value as ChatConfig["provider"],
                    )
                  }
                >
                  <option value="ollama">Ollama</option>
                  <option value="openai-compatible">OpenAI Compatible</option>
                  <option value="anthropic">
                    Anthropic (Claude / Claude 兼容代理)
                  </option>
                </select>
              </label>

              <label className="settings-field">
                <span>Base URL</span>
                <input
                  value={config.chat.baseUrl}
                  onChange={(event) =>
                    onChatConfigChange("baseUrl", event.target.value)
                  }
                  placeholder={
                    config.chat.provider === "ollama"
                      ? "http://localhost:11434"
                      : config.chat.provider === "anthropic"
                        ? "https://api.anthropic.com"
                        : "http://localhost:11434/v1"
                  }
                />
              </label>

              <label className="settings-field">
                <span>Model</span>
                <div className="settings-inline-actions">
                  <ModelCombobox
                    value={config.chat.model}
                    onChange={(v) => onChatConfigChange("model", v)}
                    models={chatProbe.models}
                    placeholder="llama3.2"
                  />
                  <button
                    type="button"
                    className="ghost-btn"
                    onClick={() => void handleProbeChat()}
                    disabled={chatProbe.status === "loading"}
                  >
                    {chatProbe.status === "loading" ? "探测中..." : "测试连接"}
                  </button>
                </div>
                {renderProbeFeedback(chatProbe)}
              </label>

              <label className="settings-field">
                <span>API Key</span>
                <input
                  type="password"
                  value={config.chat.apiKey}
                  onChange={(event) =>
                    onChatConfigChange("apiKey", event.target.value)
                  }
                  placeholder="选填"
                />
              </label>

              <label className="settings-field settings-field-full">
                <span>额外请求头 (高级,可选)</span>
                <textarea
                  className="settings-textarea"
                  rows={2}
                  value={config.chat.extraHeaders ?? ""}
                  onChange={(event) =>
                    onChatConfigChange("extraHeaders", event.target.value)
                  }
                  placeholder={`每行一个,格式 Key: Value\n例: anthropic-beta: context-1m-2025-08-07`}
                />
                <small>
                  附加到聊天请求的 HTTP header。比如 Anthropic 1M 上下文
                  beta、其他代理特殊标记。普通用户留空。
                </small>
              </label>

              <label className="settings-field settings-field-full">
                <span>Temperature: {config.chat.temperature.toFixed(1)}</span>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.1"
                  value={config.chat.temperature}
                  onChange={(event) =>
                    onChatConfigChange(
                      "temperature",
                      Number(event.target.value),
                    )
                  }
                />
              </label>

              <label className="settings-field settings-field-full">
                <span>上下文消息数量</span>
                <input
                  type="number"
                  min="1"
                  max="100"
                  value={config.chat.contextMessageLimit}
                  onChange={(event) =>
                    onChatConfigChange(
                      "contextMessageLimit",
                      Number(event.target.value),
                    )
                  }
                  placeholder="12"
                />
                <small>限制每次发送给模型的最近消息条数，范围 1-100。</small>
              </label>
              <label className="settings-field settings-field-full">
                <span>思考模式模型</span>
                <ModelCombobox
                  value={chatModeSettings.thinkModel}
                  onChange={onThinkModelChange}
                  models={chatProbe.models}
                  placeholder="deepseek-r1:8b"
                />
                <small>
                  用于"思考模式"的专用模型，建议填写推理更强但更慢的模型。复用上面"测试连接"的结果。
                </small>
              </label>
            </div>
          </section>

          <section className="settings-panel-block ai-config-panel single-column">
            <div className="section-title-row knowledge-panel-header">
              <h3>Embedding 模型</h3>
            </div>

            <div className="ai-config-fields">
              <label className="settings-field">
                <span>Provider</span>
                <select
                  value={config.embedding.provider}
                  onChange={(event) =>
                    onEmbeddingConfigChange(
                      "provider",
                      event.target.value as EmbeddingConfig["provider"],
                    )
                  }
                >
                  <option value="ollama">Ollama</option>
                  <option value="openai-compatible">OpenAI Compatible</option>
                </select>
              </label>

              <label className="settings-field">
                <span>Base URL</span>
                <input
                  value={config.embedding.baseUrl}
                  onChange={(event) =>
                    onEmbeddingConfigChange("baseUrl", event.target.value)
                  }
                  placeholder={
                    config.embedding.provider === "ollama"
                      ? "http://localhost:11434"
                      : "http://localhost:11434/v1"
                  }
                />
              </label>

              <label className="settings-field">
                <span>Model</span>
                <div className="settings-inline-actions">
                  <ModelCombobox
                    value={config.embedding.model}
                    onChange={(v) => onEmbeddingConfigChange("model", v)}
                    models={embeddingProbe.models}
                    placeholder="nomic-embed-text"
                  />
                  <button
                    type="button"
                    className="ghost-btn"
                    onClick={() => void handleProbeEmbedding()}
                    disabled={embeddingProbe.status === "loading"}
                  >
                    {embeddingProbe.status === "loading"
                      ? "探测中..."
                      : "测试连接"}
                  </button>
                </div>
                {renderProbeFeedback(embeddingProbe)}
              </label>

              <label className="settings-field">
                <span>API Key</span>
                <input
                  type="password"
                  value={config.embedding.apiKey}
                  onChange={(event) =>
                    onEmbeddingConfigChange("apiKey", event.target.value)
                  }
                  placeholder="选填"
                />
              </label>
            </div>
          </section>

          <section className="settings-panel-block ai-config-panel single-column">
            <div className="section-title-row knowledge-panel-header">
              <h3>MCP 设置</h3>
            </div>

            <div className="ai-config-fields">
              <label className="settings-field">
                <span>状态</span>
                <input
                  value={config.mcp.enabled ? "已启用" : "未启用"}
                  readOnly
                />
              </label>

              <label className="settings-field">
                <span>Base Path</span>
                <input value={config.mcp.basePath} readOnly />
              </label>

              <label className="settings-field settings-field-full">
                <span>Token</span>
                <div className="settings-inline-actions">
                  <input
                    type={isMcpTokenVisible ? "text" : "password"}
                    value={config.mcp.token}
                    readOnly
                    className="settings-token-input"
                  />
                  <button
                    type="button"
                    className="ghost-btn settings-visibility-btn"
                    onClick={() => setIsMcpTokenVisible((visible) => !visible)}
                    aria-label={isMcpTokenVisible ? "隐藏 Token" : "显示 Token"}
                    title={isMcpTokenVisible ? "隐藏 Token" : "显示 Token"}
                  >
                    {isMcpTokenVisible ? "隐藏" : "显示"}
                  </button>
                  <button
                    type="button"
                    className="ghost-btn"
                    onClick={() => void handleCopyToken()}
                  >
                    复制
                  </button>
                  <button
                    type="button"
                    className="ghost-btn"
                    onClick={() => void handleResetToken()}
                  >
                    重置
                  </button>
                </div>
                <small>
                  用于访问 MCP 接口的 Bearer Token。重置后旧 Token 会立刻失效。
                </small>
                {mcpFeedback ? (
                  <small className="settings-feedback">{mcpFeedback}</small>
                ) : null}
              </label>
            </div>
          </section>
        </div>

        <div className="settings-modal-footer">
          <div
            className="settings-modal-footer-status"
            role="status"
            aria-live="polite"
          >
            {saveStatus === "saving" && (
              <span className="settings-save-text settings-save-text--saving">
                保存中...
              </span>
            )}
            {saveStatus === "saved" && (
              <span className="settings-save-text settings-save-text--saved">
                已保存
              </span>
            )}
            {saveStatus === "error" && (
              <span className="settings-save-text settings-save-text--error">
                保存失败：{saveError || "请重试"}
              </span>
            )}
            {saveStatus === "idle" && (
              <span className="settings-save-text settings-save-text--idle">
                编辑会自动保存，也可手动点击右侧按钮重发一次
              </span>
            )}
          </div>
          <div className="settings-modal-footer-actions">
            <button
              type="button"
              className="ghost-btn"
              onClick={onClose}
              disabled={saveStatus === "saving"}
            >
              关闭
            </button>
            <button
              type="button"
              className="settings-save-btn"
              onClick={() => void handleSave()}
              disabled={saveStatus === "saving"}
            >
              {saveStatus === "saving" ? "保存中..." : "保存"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SettingsPanel;

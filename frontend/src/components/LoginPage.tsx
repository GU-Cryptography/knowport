import { useState } from "react";
import { login, register, type AuthUser } from "../api/auth";
import BrandBackdrop from "./brand/BrandBackdrop";

interface LoginPageProps {
  onAuthenticated: (user: AuthUser) => void;
  onBackToLanding?: () => void;
}

type Mode = "login" | "register";

// LoginPage：聚焦表单的认证入口。
// 由 AuthGate 子状态机渲染——视图来自 Landing CTA 跳转。
// 顶部 "← 返回首页" 让用户能回到 Landing。
export default function LoginPage({
  onAuthenticated,
  onBackToLanding,
}: LoginPageProps) {
  const [mode, setMode] = useState<Mode>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    const u = username.trim();
    if (u.length < 3 || u.length > 32) {
      setError("用户名长度须为 3-32 字符");
      return;
    }
    if (password.length < 6) {
      setError("密码至少 6 位");
      return;
    }
    setSubmitting(true);
    try {
      if (mode === "register") {
        await register(u, password);
      }
      const bundle = await login(u, password);
      onAuthenticated(bundle.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell login-shell">
      <BrandBackdrop />

      <header className="login-nav" aria-label="顶部导航">
        <button
          type="button"
          className="login-nav-back"
          onClick={onBackToLanding}
          disabled={!onBackToLanding}
        >
          <span className="login-nav-back-arrow" aria-hidden="true">
            ←
          </span>
          <span>返回首页</span>
        </button>
        <div className="login-nav-brand">
          <span className="login-nav-brand-zh">知港</span>
          <span className="login-nav-brand-en">KnowPort</span>
        </div>
        <span className="login-nav-spacer" aria-hidden="true" />
      </header>

      <main className="login-main">
        <div className="login-card">
          <div className="login-card-head">
            <span className="login-card-eyebrow">
              {mode === "login" ? "WELCOME BACK" : "GET STARTED"}
            </span>
            <h1 className="login-card-title">
              {mode === "login" ? "欢迎回来" : "创建新账号"}
            </h1>
            <p className="login-card-sub">
              {mode === "login"
                ? "登录后即可停靠到你的知识港口"
                : "几秒钟开通你的 RAG 问答账号"}
            </p>
          </div>

          <div className="login-mode-switch" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={mode === "login"}
              className={`login-mode-tab${mode === "login" ? " login-mode-tab--active" : ""}`}
              onClick={() => {
                setMode("login");
                setError(null);
              }}
              disabled={submitting}
            >
              登录
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "register"}
              className={`login-mode-tab${mode === "register" ? " login-mode-tab--active" : ""}`}
              onClick={() => {
                setMode("register");
                setError(null);
              }}
              disabled={submitting}
            >
              注册
            </button>
          </div>

          <form className="login-form" onSubmit={handleSubmit}>
            <label className="login-field">
              <span className="login-field-label">用户名</span>
              <input
                type="text"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={submitting}
                placeholder="3-32 字符"
                autoFocus
              />
            </label>

            <label className="login-field">
              <span className="login-field-label">密码</span>
              <input
                type="password"
                autoComplete={
                  mode === "login" ? "current-password" : "new-password"
                }
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={submitting}
                placeholder="至少 6 位"
              />
            </label>

            {error && (
              <div className="login-error" role="alert">
                <span>{error}</span>
              </div>
            )}

            <button
              type="submit"
              className="login-submit"
              disabled={submitting}
            >
              {submitting ? (
                <>
                  <span className="login-submit-spinner" aria-hidden="true" />
                  <span>处理中…</span>
                </>
              ) : (
                <>
                  <span>{mode === "login" ? "登录" : "注册并登录"}</span>
                  <span className="login-submit-arrow" aria-hidden="true">
                    →
                  </span>
                </>
              )}
            </button>
          </form>

          <p className="login-foot">
            {mode === "login" ? "首次使用？" : "已经拥有账号？"}
            <button
              type="button"
              className="login-foot-link"
              onClick={() => {
                setMode(mode === "login" ? "register" : "login");
                setError(null);
              }}
              disabled={submitting}
            >
              {mode === "login" ? "立即注册" : "去登录"}
            </button>
          </p>
        </div>

        <p className="login-disclaimer">© 知港 KnowPort · 开源 · 可自托管</p>
      </main>
    </div>
  );
}

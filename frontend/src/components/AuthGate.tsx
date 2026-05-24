import { useEffect, useState } from "react";
import {
  AUTH_EXPIRED_EVENT,
  clearTokens,
  fetchMe,
  getAccessToken,
  getStoredUser,
  type AuthUser,
} from "../api/auth";
import LandingPage from "./LandingPage";
import LoginPage from "./LoginPage";
import BrandBackdrop from "./brand/BrandBackdrop";

interface AuthGateProps {
  children: (user: AuthUser, logout: () => void) => React.ReactNode;
}

type AuthState =
  | { status: "checking" }
  | { status: "unauthenticated" }
  | { status: "authenticated"; user: AuthUser };

type UnauthView = "landing" | "login";

// 验证持久化 token（/api/auth/me）后渲染 children；监听 auth-expired 事件回退到 Landing。
// 在未登录态下使用 useState 子状态机切换 Landing / Login（无 react-router）。
export default function AuthGate({ children }: AuthGateProps) {
  const [state, setState] = useState<AuthState>(() => {
    if (!getAccessToken()) return { status: "unauthenticated" };
    const cached = getStoredUser();
    return cached
      ? { status: "authenticated", user: cached }
      : { status: "checking" };
  });
  const [view, setView] = useState<UnauthView>("landing");

  useEffect(() => {
    if (state.status !== "checking" && state.status !== "authenticated") return;
    if (!getAccessToken()) {
      setState({ status: "unauthenticated" });
      return;
    }
    let cancelled = false;
    fetchMe()
      .then((user) => {
        if (cancelled) return;
        if (user) {
          setState({ status: "authenticated", user });
        } else {
          clearTokens();
          setState({ status: "unauthenticated" });
        }
      })
      .catch(() => {
        if (cancelled) return;
        clearTokens();
        setState({ status: "unauthenticated" });
      });
    return () => {
      cancelled = true;
    };
    // Run once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    function handleExpired() {
      setState({ status: "unauthenticated" });
      // 用户澄清：AUTH_EXPIRED → Landing
      setView("landing");
    }
    window.addEventListener(AUTH_EXPIRED_EVENT, handleExpired);
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, handleExpired);
  }, []);

  function handleLogout() {
    clearTokens();
    setState({ status: "unauthenticated" });
    setView("landing");
  }

  if (state.status === "checking") {
    return (
      <div className="auth-shell">
        <BrandBackdrop />
        <div className="auth-checking" role="status" aria-live="polite">
          <span className="auth-checking-spinner" aria-hidden="true" />
          <span>正在校验登录状态…</span>
        </div>
      </div>
    );
  }
  if (state.status === "unauthenticated") {
    if (view === "landing") {
      return <LandingPage onStart={() => setView("login")} />;
    }
    return (
      <LoginPage
        onAuthenticated={(user) => setState({ status: "authenticated", user })}
        onBackToLanding={() => setView("landing")}
      />
    );
  }
  return <>{children(state.user, handleLogout)}</>;
}

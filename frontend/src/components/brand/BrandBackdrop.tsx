// 共享装饰层：被 LandingPage / LoginPage / AuthGate-checking 三处复用。
// 沿用 index.css 中已有的 .auth-bg-layer / .auth-blob-{1,2,3} / .auth-grid 类名，零视觉回归。

export default function BrandBackdrop() {
  return (
    <div className="auth-bg-layer" aria-hidden="true">
      <span className="auth-blob auth-blob-1" />
      <span className="auth-blob auth-blob-2" />
      <span className="auth-blob auth-blob-3" />
      <span className="auth-grid" />
    </div>
  );
}

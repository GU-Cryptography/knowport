import BrandBackdrop from "./brand/BrandBackdrop";
import {
  SCENARIOS,
  VALUE_POINTS,
  EXTERNAL_LINKS,
} from "../constants/marketing";

interface LandingPageProps {
  onStart: () => void;
}

// LandingPage v3：
// - 巨型居中 Hero + 双 CTA（试用 / 源码），借鉴 qanything 节奏但配色与版式自有。
// - Showcase 双列「场景 ↔ 能力」对比，用文字+垂直分隔代替圆形枢纽，避免直接抄。
// - 仍沿用品牌紫色渐变与 BrandBackdrop，刻意区别于参考的科技蓝/绿。
export default function LandingPage({ onStart }: LandingPageProps) {
  return (
    <div className="auth-shell landing-shell">
      <BrandBackdrop />

      <header className="landing-nav" aria-label="顶部导航">
        <a
          className="landing-nav-brand"
          href="#"
          onClick={(e) => e.preventDefault()}
        >
          <span className="landing-nav-brand-zh">知港</span>
          <span className="landing-nav-brand-en">KnowPort</span>
        </a>
        <nav className="landing-nav-links" aria-label="主导航">
          <a
            className="landing-nav-link"
            href={EXTERNAL_LINKS.github}
            target="_blank"
            rel="noreferrer noopener"
          >
            GitHub
          </a>
          <button type="button" className="landing-nav-login" onClick={onStart}>
            登录
          </button>
        </nav>
      </header>

      <main className="landing-main">
        <section className="landing-hero" aria-label="产品主张">
          <span className="landing-hero-eyebrow">
            让 RAG 跑在自己的服务器上
          </span>
          <h1 className="landing-hero-title">
            <span className="landing-hero-line">让本地知识</span>
            <span className="landing-hero-line landing-hero-line--accent">
              真正开口说话
            </span>
          </h1>
          <p className="landing-hero-sub">
            把你的文档停泊在这座港口，让 AI 在港里替你检索、阅读、引用。
            <br />
            本地优先 · 开源可部署 · 多模型自由切换。
          </p>
          <div className="landing-hero-actions">
            <button
              type="button"
              className="landing-hero-cta"
              onClick={onStart}
            >
              <span>免费试用</span>
              <span className="landing-hero-cta-arrow" aria-hidden="true">
                →
              </span>
            </button>
            <a
              className="landing-hero-cta landing-hero-cta--ghost"
              href={EXTERNAL_LINKS.github}
              target="_blank"
              rel="noreferrer noopener"
            >
              <span>立即部署</span>
              <span className="landing-hero-cta-arrow" aria-hidden="true">
                ↗
              </span>
            </a>
          </div>
          <span className="landing-hero-hint">
            注册即用 · 无需配置环境 · 喜欢就自托管
          </span>
        </section>

        <section className="landing-showcase" aria-label="场景与能力">
          <header className="landing-section-head">
            <span className="landing-section-eyebrow">SCENARIOS × ABILITY</span>
            <h2 className="landing-section-title">
              一座能停下你全部资料的 AI 知识港口
            </h2>
            <p className="landing-section-sub">
              左边是你想做的事，右边是这座港口能给你的能力。
            </p>
          </header>

          <div className="landing-showcase-grid">
            <ol className="landing-showcase-col landing-showcase-col--left">
              <div className="landing-showcase-col-head">
                <span className="landing-showcase-col-tag">场景</span>
                <span className="landing-showcase-col-sub">USE&nbsp;CASES</span>
              </div>
              {SCENARIOS.map((sc, idx) => (
                <li key={sc.label} className="landing-showcase-item">
                  <span className="landing-showcase-item-num">
                    {String(idx + 1).padStart(2, "0")}
                  </span>
                  <div className="landing-showcase-item-body">
                    <h3 className="landing-showcase-item-title">{sc.label}</h3>
                    <p className="landing-showcase-item-desc">{sc.detail}</p>
                  </div>
                </li>
              ))}
            </ol>

            <div className="landing-showcase-divider" aria-hidden="true">
              <span className="landing-showcase-divider-line" />
              <span className="landing-showcase-divider-orb">
                <span className="landing-showcase-divider-orb-zh">港</span>
              </span>
              <span className="landing-showcase-divider-line" />
            </div>

            <ol className="landing-showcase-col landing-showcase-col--right">
              <div className="landing-showcase-col-head landing-showcase-col-head--right">
                <span className="landing-showcase-col-sub">ABILITIES</span>
                <span className="landing-showcase-col-tag">能力</span>
              </div>
              {VALUE_POINTS.map((point, idx) => (
                <li key={point.title} className="landing-showcase-item">
                  <span className="landing-showcase-item-num">
                    {String(idx + 1).padStart(2, "0")}
                  </span>
                  <div className="landing-showcase-item-body">
                    <h3 className="landing-showcase-item-title">
                      {point.title}
                    </h3>
                    <p className="landing-showcase-item-desc">{point.desc}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <footer className="landing-footer" aria-label="页脚">
          <span>© 知港 KnowPort</span>
          <span className="landing-footer-sep">·</span>
          <span>开源</span>
          <span className="landing-footer-sep">·</span>
          <span>可自托管</span>
        </footer>
      </main>
    </div>
  );
}

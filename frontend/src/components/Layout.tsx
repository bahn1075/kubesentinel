import { NavLink, Link, useLocation } from "react-router-dom";
import type { ReactNode } from "react";
import type { Icon } from "@phosphor-icons/react";
import {
  SquaresFour, Siren, Stamp, ShieldCheck, BellSlash, GearSix, Flask,
} from "@phosphor-icons/react";
import { isMockMode } from "../api/client";

const NAV: { to: string; label: string; icon: Icon; end?: boolean; soon?: boolean }[] = [
  { to: "/", label: "Dashboard", icon: SquaresFour, end: true },
  { to: "/incidents", label: "Incidents", icon: Siren },
  { to: "/approvals", label: "Approvals", icon: Stamp, soon: true },
  { to: "/policies", label: "Policies", icon: ShieldCheck },
  { to: "/ignores", label: "무시 규칙", icon: BellSlash },
  { to: "/settings", label: "Settings", icon: GearSix },
];

export default function Layout({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const current = NAV.find((n) => n.end ? pathname === n.to : pathname.startsWith(n.to));
  return (
    <div className="app">
      <a className="skip-link" href="#main">본문으로 이동</a>
      <aside className="sidebar">
        <Link to="/" className="brand" aria-label="KubeSentinel 홈">
          <span className="brand-mark"><ShieldCheck size={23} weight="regular" aria-hidden /></span>
          <span>KubeSentinel<span className="brand-caption">Cluster operations</span></span>
        </Link>
        <div className="nav-heading">WORKSPACE</div>
        <nav className="nav" aria-label="주 메뉴">
          {NAV.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.end}>
              <n.icon size={18} weight="regular" aria-hidden />
              {n.label}
              {n.soon && <span className="soon">예정</span>}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <ShieldCheck size={18} aria-hidden />
          <div>Incident response<span>Kubernetes operations workspace</span></div>
        </div>
      </aside>
      <div className="workspace">
        <header className="workspace-bar">
          <div className="breadcrumb"><span>Workspace</span><span aria-hidden>/</span><span>{current?.label ?? "페이지"}</span></div>
          <span className="workspace-label">KUBERNETES / OPERATIONS</span>
        </header>
        <main className="main" id="main" tabIndex={-1}>
          {isMockMode && (
            <div className="notice">
              <Flask size={16} aria-hidden />
              <span><strong>일부 화면은 예시 데이터입니다.</strong> Incidents·Settings는 백엔드/DB와
                연동되며, Policies·Approvals는 향후 백엔드 API로 전환됩니다.</span>
            </div>
          )}
          {children}
        </main>
      </div>
    </div>
  );
}

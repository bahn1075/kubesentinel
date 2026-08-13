import { NavLink, Link } from "react-router-dom";
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
  return (
    <div className="app">
      <aside className="sidebar">
        <Link to="/" className="brand">
          <img src="/favicon.svg" className="brand-logo" alt="" /> KubeSentinel
        </Link>
        <nav className="nav">
          {NAV.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.end}>
              <n.icon size={18} weight="regular" aria-hidden />
              {n.label}
              {n.soon && <span className="soon">예정</span>}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="main">
        {isMockMode && (
          <div className="notice">
            <Flask size={16} aria-hidden />
            <span>
              <strong>일부 화면은 예시 데이터입니다.</strong> Incidents·Settings는 백엔드/DB와
              연동되며, Policies·Approvals는 향후 백엔드 API로 전환됩니다.
            </span>
          </div>
        )}
        {children}
      </main>
    </div>
  );
}

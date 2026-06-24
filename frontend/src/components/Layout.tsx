import { NavLink } from "react-router-dom";
import type { ReactNode } from "react";
import { isMockMode } from "../api/client";

const NAV = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/incidents", label: "Incidents" },
  { to: "/approvals", label: "Approvals", soon: true },
  { to: "/policies", label: "Policies" },
  { to: "/settings", label: "Settings" },
];

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="dot" /> KubeSentinel
        </div>
        <nav className="nav">
          {NAV.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.end}>
              {n.label}
              {n.soon && <span className="soon">예정</span>}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="main">
        {isMockMode && (
          <div className="banner">
            🧪 <strong>일부 화면 예시 데이터</strong> — Incidents·Settings는 백엔드/DB와 연동됩니다.
            Policies·Approvals는 아직 예시이며 향후 백엔드 API로 전환됩니다.
          </div>
        )}
        {children}
      </main>
    </div>
  );
}

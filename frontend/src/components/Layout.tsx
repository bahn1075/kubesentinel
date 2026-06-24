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
            🧪 <strong>Mock 모드</strong> — 백엔드 조회 API 연동 전입니다. 표시되는 데이터는 예시이며,
            화면 구성은 향후 기능(RCA·제안·승인·정책)을 반영합니다.
          </div>
        )}
        {children}
      </main>
    </div>
  );
}

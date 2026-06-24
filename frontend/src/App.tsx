import { Routes, Route } from "react-router-dom";
import Layout from "./components/Layout";
import Dashboard from "./pages/Dashboard";
import Incidents from "./pages/Incidents";
import IncidentDetail from "./pages/IncidentDetail";
import Approvals from "./pages/Approvals";
import Policies from "./pages/Policies";
import Settings from "./pages/Settings";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/incidents" element={<Incidents />} />
        <Route path="/incidents/:id" element={<IncidentDetail />} />
        <Route path="/approvals" element={<Approvals />} />
        <Route path="/policies" element={<Policies />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<p>페이지를 찾을 수 없습니다.</p>} />
      </Routes>
    </Layout>
  );
}

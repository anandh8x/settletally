import { LandingPage } from "./LandingPage";
import { WorkspacePage } from "./WorkspacePage";

export function App() {
  const path = window.location.pathname.replace(/\/+$/, "") || "/";
  return path === "/app" ? <WorkspacePage /> : <LandingPage />;
}

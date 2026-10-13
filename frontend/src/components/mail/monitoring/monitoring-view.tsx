import { MonitoringPage } from "./monitoring-page";
import { ServerHealthPanel } from "../shared/server-health-panel";

export function MonitoringView() {
  return (
    <>
      <MonitoringPage />
      <ServerHealthPanel />
    </>
  );
}

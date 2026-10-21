import { AppShell } from "@/components/layout/AppShell";
import { api } from "@/lib/api";
import { BackupsPage } from "@/pages/Backups";
import { CreateGuestPage } from "@/pages/CreateGuest";
import { HAPage } from "@/pages/HA";
import { IdentityPage } from "@/pages/Identity";
import { FirewallPage } from "@/pages/Firewall";
import { DatacenterPage } from "@/pages/Datacenter";
import { SDNPage } from "@/pages/SDN";
import { CephPage } from "@/pages/Ceph";
import { OverviewPage } from "@/pages/Overview";
import { TopNPage } from "@/pages/TopN";
import { RecipesPage } from "@/pages/Scripts";
import { AuditPage } from "@/pages/Audit";
import { HistoryPage } from "@/pages/History";
import { GuestDetailPage } from "@/pages/GuestDetail";
import { LoginPage } from "@/pages/Login";
import { NodeDetailPage } from "@/pages/NodeDetail";
import { SettingsPage } from "@/pages/Settings";
import { SetupPage } from "@/pages/Setup";
import { StorageBrowserPage } from "@/pages/StorageBrowser";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/setup",
  component: SetupPage,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  beforeLoad: async () => {
    const me = await api.me();
    if (me.setupRequired) throw redirect({ to: "/setup" });
    if (!me.user) throw redirect({ to: "/login" });
    return {
      username: me.user.username,
      kind: me.user.kind ?? "local",
      permissions: me.permissions ?? {},
      tokenScoped: me.permissionsFrom === "token",
      clusterConfigured: me.clusterConfigured,
    };
  },
  component: function AuthLayout() {
    const { username, kind, permissions, tokenScoped, clusterConfigured } = appRoute.useRouteContext();
    return (
      <AppShell username={username} kind={kind} permissions={permissions} tokenScoped={tokenScoped} clusterConfigured={clusterConfigured} />
    );
  },
});

const indexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  component: OverviewPage,
});

const topNRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/top",
  component: TopNPage,
});

const recipesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/recipes",
  component: RecipesPage,
});

const scriptsRedirect = createRoute({
  getParentRoute: () => appRoute,
  path: "/scripts",
  beforeLoad: () => {
    throw redirect({ to: "/recipes" });
  },
  component: () => null,
});

const auditRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/audit",
  component: AuditPage,
});

const historyRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/history",
  component: HistoryPage,
});

const nodeRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/nodes/$node",
  validateSearch: (s: Record<string, unknown>): { tab?: string } =>
    typeof s.tab === "string" ? { tab: s.tab } : {},
  component: NodeDetailPage,
});

const guestRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/guests/$vmid",
  validateSearch: (s: Record<string, unknown>): { node: string | undefined; type: string | undefined; tab?: string } => ({
    node: typeof s.node === "string" ? s.node : undefined,
    type: typeof s.type === "string" ? s.type : undefined,
    tab: typeof s.tab === "string" ? s.tab : undefined,
  }),
  component: GuestDetailPage,
});

const storageRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/storage/$storage",
  validateSearch: (s: Record<string, unknown>) => ({
    node: typeof s.node === "string" ? s.node : undefined,
  }),
  component: StorageBrowserPage,
});

const createRouteDef = createRoute({
  getParentRoute: () => appRoute,
  path: "/create",
  validateSearch: (s: Record<string, unknown>): { import?: "1"; storage?: string; node?: string; volume?: string } => {
    const out: { import?: "1"; storage?: string; node?: string; volume?: string } = {};
    if (s.import === "1" || s.import === true) out.import = "1";
    if (typeof s.storage === "string") out.storage = s.storage;
    if (typeof s.node === "string") out.node = s.node;
    if (typeof s.volume === "string") out.volume = s.volume;
    return out;
  },
  component: CreateGuestPage,
});

const backupsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/backups",
  validateSearch: (s: Record<string, unknown>): { tab?: "jobs" | "coverage" | "replication" } => {
    if (s.tab === "coverage" || s.tab === "replication" || s.tab === "jobs") {
      return { tab: s.tab };
    }
    return {};
  },
  component: BackupsPage,
});

const haRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/ha",
  component: HAPage,
});

const identityRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/identity",
  component: IdentityPage,
});

const firewallRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/firewall",
  validateSearch: (s: Record<string, unknown>): { tab?: "node"; node?: string } => {
    const out: { tab?: "node"; node?: string } = {};
    if (s.tab === "node") out.tab = "node";
    if (typeof s.node === "string") out.node = s.node;
    return out;
  },
  component: FirewallPage,
});

const datacenterRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/datacenter",
  validateSearch: (s: Record<string, unknown>): { tab?: "storage" | "config"; section?: string } => ({
    ...(s.tab === "storage" || s.tab === "config" ? { tab: s.tab } : {}),
    ...(typeof s.section === "string" ? { section: s.section } : {}),
  }),
  // The datacenter overview lives at "/"; this route only hosts its other tabs.
  beforeLoad: ({ search }) => {
    if (!search.tab) throw redirect({ to: "/" });
  },
  component: DatacenterPage,
});

const sdnRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/sdn",
  validateSearch: (s: Record<string, unknown>): { tab?: "runtime"; node?: string } => {
    const out: { tab?: "runtime"; node?: string } = {};
    if (s.tab === "runtime") out.tab = "runtime";
    if (typeof s.node === "string") out.node = s.node;
    return out;
  },
  component: SDNPage,
});

const cephRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/ceph",
  validateSearch: (s: Record<string, unknown>): { node?: string } =>
    typeof s.node === "string" ? { node: s.node } : {},
  component: CephPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/settings",
  component: SettingsPage,
});

const routeTree = rootRoute.addChildren([
  setupRoute,
  loginRoute,
  appRoute.addChildren([indexRoute, topNRoute, recipesRoute, scriptsRedirect, auditRoute, historyRoute, nodeRoute, guestRoute, storageRoute, createRouteDef, backupsRoute, haRoute, identityRoute, firewallRoute, datacenterRoute, sdnRoute, cephRoute, settingsRoute]),
]);

const router = createRouter({
  routeTree,
  defaultPendingComponent: () => (
    <div className="grid h-full place-items-center text-sm text-ink-dim">Loading Hypervane…</div>
  ),
  defaultErrorComponent: ({ error }) => (
    <div className="grid h-full place-items-center p-8 text-sm text-rose-400">{error.message}</div>
  ),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

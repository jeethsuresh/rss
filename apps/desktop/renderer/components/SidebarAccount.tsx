export type SidebarAccountProps = {
  name: string;
  signOutLabel: string;
  signingOut: boolean;
  connectionUnavailable: boolean;
  onSignOut: () => void;
};

export function SidebarAccount({
  account,
  onOpenPermissions,
}: {
  account: SidebarAccountProps;
  onOpenPermissions: () => void;
}) {
  return <div className="sidebar-account">
    <div className="sidebar-account-row">
      <span className="sidebar-account-name" title={account.name}>{account.name}</span>
      <button type="button" className="btn sidebar-account-action" aria-label="App permissions and installation" title="App permissions and installation" onClick={onOpenPermissions}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M12 3v12m-5-5 5 5 5-5M4 16v4h16v-4" /></svg>
      </button>
      <button type="button" className="btn sidebar-account-action" aria-label={account.signingOut ? "Signing out…" : account.signOutLabel} title={account.signOutLabel} disabled={account.signingOut} onClick={account.onSignOut}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 4H4v16h5m5-13 5 5-5 5m-6-5h15" /></svg>
      </button>
    </div>
    {account.connectionUnavailable && <p className="sidebar-account-status" role="status">Connection unavailable. Saved content is available on this device.</p>}
  </div>;
}

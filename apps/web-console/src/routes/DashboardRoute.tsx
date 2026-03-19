export function DashboardRoute() {
  return (
    <section>
      <h1 style={{ marginTop: 0 }}>Operations Dashboard</h1>
      <p>Phase 0 dashboard shell with workflow, health, and audit summary cards.</p>
      <div style={{ display: 'grid', gap: 16, gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))' }}>
        <div style={{ border: '1px solid #1f2937', borderRadius: 8, padding: 16 }}>
          <h3>Workflow Queue</h3>
          <p>0 running / 0 failed</p>
        </div>
        <div style={{ border: '1px solid #1f2937', borderRadius: 8, padding: 16 }}>
          <h3>Node Health</h3>
          <p>Control-plane reachable</p>
        </div>
        <div style={{ border: '1px solid #1f2937', borderRadius: 8, padding: 16 }}>
          <h3>Audit Stream</h3>
          <p>No events yet</p>
        </div>
      </div>
    </section>
  );
}

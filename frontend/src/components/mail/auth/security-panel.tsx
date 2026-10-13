import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
export function SecurityPanel() {
  const client = useQueryClient();
  const sessions = useQuery({ queryKey: ["sessions"], queryFn: api.sessions });
  const [password, setPassword] = useState("");
  const [next, setNext] = useState("");
  const [code, setCode] = useState("");
  const [secret, setSecret] = useState("");
  const [recovery, setRecovery] = useState<string[]>([]);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setMessage("");
    try {
      await fn();
      await client.invalidateQueries();
      setMessage("Security settings updated.");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Operation failed.");
    } finally {
      setBusy(false);
      setPassword("");
      setNext("");
      setCode("");
    }
  };
  return (
    <section className="space-y-4 max-w-2xl">
      <h2 className="section-heading">Account security</h2>
      <label>
        Current password
        <input
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </label>
      <label>
        New password
        <input
          type="password"
          minLength={12}
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
        />
      </label>
      <Button
        disabled={busy || !password || next.length < 12}
        onClick={() =>
          void run(async () => {
            await api.changePassword(password, next);
          })
        }
      >
        Change password and sign out all sessions
      </Button>
      <h3>Authenticator app</h3>
      <Button
        disabled={busy || !password}
        onClick={() =>
          void run(async () => {
            const result = await api.enrollTotp(password);
            setSecret(result.secret);
            setRecovery([]);
          })
        }
      >
        Start MFA enrollment
      </Button>
      {secret && (
        <p className="break-all">
          Enter this secret in your authenticator app: <code>{secret}</code>
        </p>
      )}
      <label>
        Authenticator or recovery code
        <input
          autoComplete="one-time-code"
          value={code}
          onChange={(e) => setCode(e.target.value)}
        />
      </label>
      <div className="flex gap-3">
        <Button
          disabled={busy || !code || !secret}
          onClick={() =>
            void run(async () => {
              const result = await api.verifyTotp(code);
              setRecovery(result.recoveryCodes);
              setSecret("");
            })
          }
        >
          Verify enrollment
        </Button>
        <Button
          variant="outline"
          disabled={busy || !code || !password}
          onClick={() =>
            void run(async () => {
              await api.disableTotp(password, code);
              setRecovery([]);
              setSecret("");
            })
          }
        >
          Disable MFA
        </Button>
      </div>
      {recovery.length > 0 && (
        <div>
          <p>Save these recovery codes securely. Each can be used once.</p>
          <pre>{recovery.join("\n")}</pre>
        </div>
      )}
      <h3>Active sessions</h3>
      {sessions.isError && <p>Session list unavailable.</p>}
      {(sessions.data ?? []).map((s) => (
        <p key={s.id}>
          {s.ip} · {s.userAgent} · expires {s.expiresAt}
        </p>
      ))}
      <div className="flex gap-3">
        <Button
          disabled={busy}
          onClick={() =>
            void run(async () => {
              await api.rotateSession();
            })
          }
        >
          Rotate current session
        </Button>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() =>
            void run(async () => {
              await api.revokeSessions();
            })
          }
        >
          Sign out all sessions
        </Button>
      </div>
      {message && <p role="status">{message}</p>}
    </section>
  );
}

"use client";

// Public route (no SessionGate) — crm_be mounts GET .../token/{token}
// fully public and POST .../accept behind OptionalMiddleware (TD phase
// 1 §6.1: "publik atau terautentikasi"). Route is /invitations/accept
// (not /invite) to match the link crm_be actually emails out —
// internal/invitation/usecase.go builds
// "%s/invitations/accept?token=%s", same convention as
// /verify-email and /reset-password. Not covered by the Claude Design
// mockup at all (its sectionIs*/screenIs* states never include
// anything like this — it only modeled the already-authenticated app
// shell and the 5 auth screens from #31), so this follows the same
// centered-card visual language as login/register rather than a
// specific design reference.
import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { FormErrorBanner } from "@/components/form-error-banner";
import {
  acceptInvitation,
  getInvitationTokenInfo,
  type InvitationTokenInfo,
} from "@/lib/invitations";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";

const MIN_PASSWORD_LENGTH = 12;

type Status = "loading" | "ready" | "invalid" | "accepted";

export function InviteForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";

  const [status, setStatus] = useState<Status>(token ? "loading" : "invalid");
  const [info, setInfo] = useState<InvitationTokenInfo | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [fullName, setFullName] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    getInvitationTokenInfo(token)
      .then((result) => {
        if (!cancelled) {
          setInfo(result);
          setStatus("ready");
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setLoadError(globalMessage(err));
          setStatus("invalid");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function handleSubmitNewUser(event: FormEvent) {
    event.preventDefault();
    setSubmitError(null);
    setFieldErrors({});
    if (password.length < MIN_PASSWORD_LENGTH) {
      setFieldErrors({ password: `Minimal ${MIN_PASSWORD_LENGTH} karakter.` });
      return;
    }
    setSubmitting(true);
    try {
      await acceptInvitation({ token, fullName, password });
      setStatus("accepted");
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) setFieldErrors(fields);
      else setSubmitError(globalMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleAcceptExisting() {
    setSubmitError(null);
    setSubmitting(true);
    try {
      await acceptInvitation({ token });
      router.push("/");
    } catch (err) {
      // A genuinely unauthenticated visitor gets redirected to /login by
      // apiFetch's own 401 handling before this ever runs; this branch
      // covers everything else (e.g. logged in as a different account).
      setSubmitError(globalMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  if (status === "loading") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Memuat undangan…</CardTitle>
        </CardHeader>
      </Card>
    );
  }

  if (status === "invalid") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Tautan undangan tidak valid</CardTitle>
          <CardDescription>{loadError ?? "Tautan ini tidak valid atau sudah kedaluwarsa."}</CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/login" className="text-sm underline underline-offset-4">
            Kembali ke halaman masuk
          </Link>
        </CardContent>
      </Card>
    );
  }

  if (status === "accepted") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Undangan diterima</CardTitle>
          <CardDescription>Akun Anda sudah aktif di {info?.organization_name}.</CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/login" className="text-sm underline underline-offset-4">
            Masuk sekarang
          </Link>
        </CardContent>
      </Card>
    );
  }

  // status === "ready"
  if (info?.user_exists) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Undangan ke {info.organization_name}</CardTitle>
          <CardDescription>
            Akun dengan email {info.email} sudah ada. Masuk terlebih dahulu bila belum, lalu klik
            tombol di bawah untuk menerima undangan ini.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <FormErrorBanner message={submitError} />
          <Button type="button" disabled={submitting} onClick={handleAcceptExisting}>
            {submitting ? "Memproses…" : "Terima undangan"}
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Belum masuk?{" "}
            <Link href="/login" className="underline underline-offset-4">
              Masuk dulu
            </Link>
            , lalu buka tautan undangan ini lagi.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Undangan ke {info?.organization_name}</CardTitle>
        <CardDescription>Lengkapi data untuk membuat akun {info?.email}.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmitNewUser} className="flex flex-col gap-4">
          <FormErrorBanner message={submitError} />

          <div className="flex flex-col gap-2">
            <Label htmlFor="invite-full-name">Nama lengkap</Label>
            <Input
              id="invite-full-name"
              required
              autoFocus
              autoComplete="name"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
            />
            <FieldError message={fieldErrors.full_name} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="invite-password">Password</Label>
            <Input
              id="invite-password"
              type="password"
              required
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <FieldError message={fieldErrors.password} />
          </div>

          <Button type="submit" disabled={submitting}>
            {submitting ? "Memproses…" : "Buat akun"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

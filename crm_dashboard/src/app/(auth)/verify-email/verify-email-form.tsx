"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
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
import { FormErrorBanner } from "@/components/form-error-banner";
import { verifyEmail, resendVerification } from "@/lib/auth";
import { globalMessage } from "@/lib/auth-errors";

type Status = "verifying" | "verified" | "failed";

export function VerifyEmailForm() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const [status, setStatus] = useState<Status>(token ? "verifying" : "failed");
  const [error, setError] = useState<string | null>(null);
  const [resendEmail, setResendEmail] = useState("");
  const [resendSent, setResendSent] = useState(false);

  useEffect(() => {
    if (!token) return;
    verifyEmail(token)
      .then(() => setStatus("verified"))
      .catch((err) => {
        setStatus("failed");
        setError(globalMessage(err));
      });
  }, [token]);

  async function handleResend(event: FormEvent) {
    event.preventDefault();
    await resendVerification(resendEmail).catch(() => {
      // resend always answers 202 regardless of outcome (anti-enumeration,
      // TD §6.3) — nothing meaningful to show on failure either.
    });
    setResendSent(true);
  }

  if (status === "verifying") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Memverifikasi email...</CardTitle>
        </CardHeader>
      </Card>
    );
  }

  if (status === "verified") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Email terverifikasi</CardTitle>
          <CardDescription>Akun Anda sudah aktif.</CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/login" className="text-sm underline underline-offset-4">
            Masuk sekarang
          </Link>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Verifikasi gagal</CardTitle>
        <CardDescription>
          Tautan verifikasi tidak valid atau sudah kedaluwarsa. Kirim ulang ke email Anda.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {resendSent ? (
          <p className="text-sm text-muted-foreground">
            Bila email Anda terdaftar, tautan verifikasi baru sudah dikirim.
          </p>
        ) : (
          <form onSubmit={handleResend} className="flex flex-col gap-4">
            <FormErrorBanner message={error} />
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                required
                value={resendEmail}
                onChange={(event) => setResendEmail(event.target.value)}
              />
            </div>
            <Button type="submit">Kirim ulang</Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}

"use client";

import { useState, type FormEvent } from "react";
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
import { resetPassword } from "@/lib/auth";
import { globalMessage } from "@/lib/auth-errors";

const MIN_PASSWORD_LENGTH = 12;

export function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldError(null);

    if (password.length < MIN_PASSWORD_LENGTH) {
      setFieldError(`Minimal ${MIN_PASSWORD_LENGTH} karakter.`);
      return;
    }
    if (password !== confirmPassword) {
      setFieldError("Konfirmasi password tidak cocok.");
      return;
    }
    if (!token) {
      setError("Tautan reset password tidak valid.");
      return;
    }

    setLoading(true);
    try {
      await resetPassword(token, password);
      router.push("/login");
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  if (!token) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Tautan tidak valid</CardTitle>
          <CardDescription>Minta tautan reset password baru.</CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/forgot-password" className="text-sm underline underline-offset-4">
            Lupa password
          </Link>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Atur ulang password</CardTitle>
        <CardDescription>Masukkan password baru Anda.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormErrorBanner message={error} />

          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Password baru</Label>
            <Input
              id="password"
              type="password"
              required
              autoComplete="new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="confirm_password">Konfirmasi password</Label>
            <Input
              id="confirm_password"
              type="password"
              required
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
            />
            <FieldError message={fieldError ?? undefined} />
          </div>

          <Button type="submit" disabled={loading}>
            {loading ? "Memproses..." : "Simpan password baru"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

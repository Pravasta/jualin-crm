"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
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
import { register } from "@/lib/auth";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";

// minPasswordLength mirrors crm_be/internal/auth/usecase.go — client-side
// check is UX only, the server is the single source of truth.
const MIN_PASSWORD_LENGTH = 12;

export default function RegisterPage() {
  const [organizationName, setOrganizationName] = useState("");
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [submittedEmail, setSubmittedEmail] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});

    if (password.length < MIN_PASSWORD_LENGTH) {
      setFieldErrors({ password: `Minimal ${MIN_PASSWORD_LENGTH} karakter.` });
      return;
    }

    setLoading(true);
    try {
      await register({ organizationName, fullName, email, password });
      setSubmittedEmail(email);
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) {
        setFieldErrors(fields);
      } else {
        setError(globalMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }

  if (submittedEmail) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Cek email Anda</CardTitle>
          <CardDescription>
            Kami mengirim tautan verifikasi ke <strong>{submittedEmail}</strong>. Buka tautan itu
            untuk mengaktifkan akun Anda.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/login" className="text-sm underline underline-offset-4">
            Kembali ke halaman masuk
          </Link>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Daftar</CardTitle>
        <CardDescription>Buat organization baru di Jualin CRM.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormErrorBanner message={error} />

          <div className="flex flex-col gap-2">
            <Label htmlFor="organization_name">Nama organization</Label>
            <Input
              id="organization_name"
              required
              value={organizationName}
              onChange={(event) => setOrganizationName(event.target.value)}
            />
            <FieldError message={fieldErrors.organization_name} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="full_name">Nama lengkap</Label>
            <Input
              id="full_name"
              required
              autoComplete="name"
              value={fullName}
              onChange={(event) => setFullName(event.target.value)}
            />
            <FieldError message={fieldErrors.full_name} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
            <FieldError message={fieldErrors.email} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              required
              autoComplete="new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            <FieldError message={fieldErrors.password} />
          </div>

          <Button type="submit" disabled={loading}>
            {loading ? "Memproses..." : "Daftar"}
          </Button>

          <p className="text-center text-sm text-muted-foreground">
            Sudah punya akun?{" "}
            <Link href="/login" className="underline underline-offset-4">
              Masuk
            </Link>
          </p>
        </form>
      </CardContent>
    </Card>
  );
}

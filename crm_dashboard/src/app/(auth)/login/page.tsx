"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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
import { login } from "@/lib/auth";
import { globalMessage, organizationsFrom, type OrganizationOption } from "@/lib/auth-errors";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [organizations, setOrganizations] = useState<OrganizationOption[] | null>(null);
  const [organizationId, setOrganizationId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await login({
        email,
        password,
        organizationId: organizations ? organizationId : undefined,
      });
      router.push("/");
    } catch (err) {
      // 409 organization_selection_required (ADR-007) — user has >1
      // active membership. Switch this same form into org-picker mode
      // instead of navigating away; email/password stay in state so
      // the second login call can reuse them.
      const orgs = organizationsFrom(err);
      if (orgs) {
        setOrganizations(orgs);
      } else {
        setError(globalMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Masuk</CardTitle>
        <CardDescription>
          {organizations
            ? "Akun Anda terdaftar di lebih dari satu organization."
            : "Masuk ke akun Jualin CRM Anda."}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormErrorBanner message={error} />

          {organizations ? (
            <div className="flex flex-col gap-2">
              <Label htmlFor="organization">Organization</Label>
              <select
                id="organization"
                required
                value={organizationId}
                onChange={(event) => setOrganizationId(event.target.value)}
                className="flex h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm dark:bg-input/30"
              >
                <option value="" disabled>
                  Pilih organization
                </option>
                {organizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <>
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
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
              </div>
            </>
          )}

          <Button type="submit" disabled={loading || (organizations !== null && !organizationId)}>
            {loading ? "Memproses..." : organizations ? "Lanjutkan" : "Masuk"}
          </Button>

          {!organizations && (
            <div className="flex justify-between text-sm text-muted-foreground">
              <Link href="/forgot-password" className="underline underline-offset-4">
                Lupa password?
              </Link>
              <Link href="/register" className="underline underline-offset-4">
                Daftar organization baru
              </Link>
            </div>
          )}
        </form>
      </CardContent>
    </Card>
  );
}

import { PlaceholderScreen } from "@/components/placeholder-screen";

// Metrik home mengonsumsi GET /v1/metrics/* — dibangun di issue #35.
export default function BerandaPage() {
  return (
    <PlaceholderScreen
      title="Beranda"
      description="Ringkasan metrik — lead masuk, jumlah per status, lead belum ter-assign, dan conversion rate."
      issue={35}
    />
  );
}

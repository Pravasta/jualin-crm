import 'package:flutter/material.dart';

/// Design tokens from Claude Design's Phase 5 spec
/// (`docs/phases/05-employee-mobile/design-brief.md` §12 deliverable —
/// project "Employee mobile design spec"). Base colors are inherited from
/// `crm_dashboard` and locked (design brief §4.1); status/semantic colors
/// are new, derived from the same hue family in OKLCH.
///
/// **Every text/background pair below was recomputed independently**
/// (WCAG 2.1 relative-luminance formula, not the design tool's own
/// printed numbers) before being accepted — issue #70's acceptance
/// criterion is "dihitung, bukan dikira". All pairs pass 4.5:1 (normal
/// text); three of the design's own printed ratios were measurably off
/// (still comfortably passing) — recomputed values are what's documented
/// here. See `docs/phases/05-employee-mobile/notes.md`'s `## #70` for
/// the full comparison table. Unlike Phase 3's #40, no color needed
/// correcting this time — every pair the design produced already clears
/// AA once verified.
class AppColors {
  AppColors._();

  // --- Base (locked from crm_dashboard, design brief §4.1) ---

  /// Primary action button background. White text on this: **5.05:1**
  /// (design's own figure: 4.83 — recomputed here, still passes either
  /// way).
  static const primary = Color(0xFFCA3C00);

  /// Accented text/icons on white. Never used as a background for white
  /// text (that's [primary]). **7.03:1** on white.
  static const accentStrong = Color(0xFFA72B00);

  /// Primary text on white. **19.80:1**.
  static const foreground = Color(0xFF0A0A0A);

  /// Secondary/metadata text on white. **4.74:1** — passes AA but sits
  /// right at the edge; the design spec itself flags this as
  /// borderline and restricts it to metadata ≥13px, never critical
  /// information. Respected here, not just noted.
  static const mutedForeground = Color(0xFF737373);

  /// Decorative only (dividers) — never used for text, so no contrast
  /// requirement applies.
  static const border = Color(0xFFE5E5E5);

  /// App background, bottom nav, cache banner backdrop. Decorative.
  static const surfaceSunken = Color(0xFFF5F5F5);

  static const surface = Colors.white;

  // --- Semantic (new, same hue family) ---

  /// Text on [dangerTint]. **6.29:1**.
  static const danger = Color(0xFFB00A1D);
  static const dangerTint = Color(0xFFFFEBE8);

  /// Text on [warningTint]. **6.65:1** (design's own figure: 7.6 —
  /// recomputed, still passes).
  static const warning = Color(0xFF7A4A00);
  static const warningTint = Color(0xFFFDF0DC);

  /// Text on [successTint]. **7.16:1** (design's own figure: 7.0 —
  /// recomputed, still passes).
  static const success = Color(0xFF005F0E);
  static const successTint = Color(0xFFE6F8E6);
}

/// 4pt scale (design brief §1.5) — every margin/padding/gap in this app
/// should come from here, not a literal number, so spacing stays
/// consistent as more screens land.
class AppSpacing {
  AppSpacing._();

  static const double space4 = 4;
  static const double space8 = 8;
  static const double space12 = 12;
  static const double space16 = 16;
  static const double space20 = 20;
  static const double space24 = 24;
  static const double space32 = 32;
  static const double space40 = 40;
}

class AppRadius {
  AppRadius._();

  static const double button = 12;
  static const double card = 12;

  /// Status badges — fully rounded regardless of height.
  static const double pill = 100;
  static const double dialog = 20;
}

/// Design brief §1.5 — Android's own minimum is 44dp; this app's floor is
/// slightly above it.
const double kMinTouchTarget = 48;

/// Roboto — Android's system font (design brief §1.4: legible, and never
/// adds download weight on a low-end device since it's already there).
/// Deliberately NOT `google_fonts` — Flutter's Material widgets already
/// resolve to Roboto-equivalent metrics on an Android target with no
/// font family specified, and TD phase 5 has no dependency budget for a
/// font package this app doesn't actually need.
class AppTextStyles {
  AppTextStyles._();

  static const screenTitle = TextStyle(
    fontSize: 28,
    height: 34 / 28,
    fontWeight: FontWeight.w700,
    color: AppColors.foreground,
  );

  static const cardTitle = TextStyle(
    fontSize: 17,
    height: 24 / 17,
    fontWeight: FontWeight.w600,
    color: AppColors.foreground,
  );

  static const buttonLabel = TextStyle(
    fontSize: 16,
    height: 24 / 16,
    fontWeight: FontWeight.w600,
  );

  static const body = TextStyle(
    fontSize: 15,
    height: 22 / 15,
    fontWeight: FontWeight.w400,
    color: AppColors.foreground,
  );

  static const metadata = TextStyle(
    fontSize: 13,
    height: 18 / 13,
    fontWeight: FontWeight.w400,
    color: AppColors.mutedForeground,
  );
}

class AppTheme {
  AppTheme._();

  static ThemeData get light {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: AppColors.primary,
      brightness: Brightness.light,
      primary: AppColors.primary,
      onPrimary: Colors.white,
      surface: AppColors.surface,
      onSurface: AppColors.foreground,
      error: AppColors.danger,
      onError: Colors.white,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: AppColors.surface,
      appBarTheme: const AppBarTheme(
        backgroundColor: AppColors.surface,
        foregroundColor: AppColors.foreground,
        elevation: 0,
        titleTextStyle: AppTextStyles.cardTitle,
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: AppColors.primary,
          foregroundColor: Colors.white,
          minimumSize: const Size.fromHeight(52),
          textStyle: AppTextStyles.buttonLabel,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.button),
          ),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: false,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.space16,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: AppColors.border, width: 1.5),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: AppColors.border, width: 1.5),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: AppColors.primary, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: AppColors.danger, width: 1.5),
        ),
      ),
      extensions: const <ThemeExtension<dynamic>>[AppColorsExtension.light],
    );
  }
}

/// Tokens `ColorScheme` has no slot for — read via
/// `Theme.of(context).extension<AppColorsExtension>()!`.
@immutable
class AppColorsExtension extends ThemeExtension<AppColorsExtension> {
  final Color accentStrong;
  final Color mutedForeground;
  final Color border;
  final Color surfaceSunken;
  final Color warning;
  final Color warningTint;
  final Color success;
  final Color successTint;
  final Color dangerTint;

  const AppColorsExtension({
    required this.accentStrong,
    required this.mutedForeground,
    required this.border,
    required this.surfaceSunken,
    required this.warning,
    required this.warningTint,
    required this.success,
    required this.successTint,
    required this.dangerTint,
  });

  static const light = AppColorsExtension(
    accentStrong: AppColors.accentStrong,
    mutedForeground: AppColors.mutedForeground,
    border: AppColors.border,
    surfaceSunken: AppColors.surfaceSunken,
    warning: AppColors.warning,
    warningTint: AppColors.warningTint,
    success: AppColors.success,
    successTint: AppColors.successTint,
    dangerTint: AppColors.dangerTint,
  );

  @override
  AppColorsExtension copyWith({
    Color? accentStrong,
    Color? mutedForeground,
    Color? border,
    Color? surfaceSunken,
    Color? warning,
    Color? warningTint,
    Color? success,
    Color? successTint,
    Color? dangerTint,
  }) {
    return AppColorsExtension(
      accentStrong: accentStrong ?? this.accentStrong,
      mutedForeground: mutedForeground ?? this.mutedForeground,
      border: border ?? this.border,
      surfaceSunken: surfaceSunken ?? this.surfaceSunken,
      warning: warning ?? this.warning,
      warningTint: warningTint ?? this.warningTint,
      success: success ?? this.success,
      successTint: successTint ?? this.successTint,
      dangerTint: dangerTint ?? this.dangerTint,
    );
  }

  @override
  AppColorsExtension lerp(
    covariant ThemeExtension<AppColorsExtension>? other,
    double t,
  ) {
    if (other is! AppColorsExtension) return this;
    return AppColorsExtension(
      accentStrong: Color.lerp(accentStrong, other.accentStrong, t)!,
      mutedForeground: Color.lerp(mutedForeground, other.mutedForeground, t)!,
      border: Color.lerp(border, other.border, t)!,
      surfaceSunken: Color.lerp(surfaceSunken, other.surfaceSunken, t)!,
      warning: Color.lerp(warning, other.warning, t)!,
      warningTint: Color.lerp(warningTint, other.warningTint, t)!,
      success: Color.lerp(success, other.success, t)!,
      successTint: Color.lerp(successTint, other.successTint, t)!,
      dangerTint: Color.lerp(dangerTint, other.dangerTint, t)!,
    );
  }
}

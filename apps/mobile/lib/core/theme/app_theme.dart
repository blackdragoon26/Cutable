import 'package:flutter/material.dart';

/// Light and dark themes mirroring apps/frontend/app/globals.css.
///
/// Light is the web app's default "paper" look: cream background, pink
/// accent, sage-green secondary. Dark mirrors the CSS custom properties
/// (--background, --foreground, --surface, --border, --muted, --accent).
class AppTheme {
  AppTheme._();

  static const Color lightBackground = Color(0xFFF6F6F3);
  static const Color lightSurface = Color(0xFFFFFFFF);
  static const Color lightBorder = Color(0xFFE7E5E4);
  static const Color lightForeground = Color(0xFF1C1917);
  static const Color lightMuted = Color(0xFF78716C);
  static const Color accentPink = Color(0xFFE6538B);
  static const Color secondarySage = Color(0xFF557B6F);

  static const Color darkBackground = Color(0xFF09090B);
  static const Color darkSurface = Color(0xFF141416);
  static const Color darkBorder = Color(0xFF2B2B30);
  static const Color darkForeground = Color(0xFFFAFAF9);
  static const Color darkMuted = Color(0xFFA8A29E);
  static const Color darkAccent = Color(0xFFF05E95);

  static ThemeData light() {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: accentPink,
      brightness: Brightness.light,
      primary: accentPink,
      secondary: secondarySage,
      surface: lightSurface,
    );
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: lightBackground,
      appBarTheme: const AppBarTheme(
        backgroundColor: lightBackground,
        foregroundColor: lightForeground,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
      ),
      cardTheme: CardThemeData(
        color: lightSurface,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: const BorderSide(color: lightBorder),
        ),
      ),
      dividerTheme: const DividerThemeData(color: lightBorder),
      textTheme: const TextTheme().apply(
        bodyColor: lightForeground,
        displayColor: lightForeground,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accentPink,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: lightForeground,
          side: const BorderSide(color: lightBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: lightSurface,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: lightBorder),
        ),
      ),
    );
  }

  static ThemeData dark() {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: darkAccent,
      brightness: Brightness.dark,
      primary: darkAccent,
      secondary: secondarySage,
      surface: darkSurface,
    );
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: darkBackground,
      appBarTheme: const AppBarTheme(
        backgroundColor: darkBackground,
        foregroundColor: darkForeground,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
      ),
      cardTheme: CardThemeData(
        color: darkSurface,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: const BorderSide(color: darkBorder),
        ),
      ),
      dividerTheme: const DividerThemeData(color: darkBorder),
      textTheme: const TextTheme().apply(
        bodyColor: darkForeground,
        displayColor: darkForeground,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: darkAccent,
          foregroundColor: Colors.black,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: darkForeground,
          side: const BorderSide(color: darkBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: darkSurface,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: darkBorder),
        ),
      ),
    );
  }

  /// Muted/secondary text color for the active brightness, mirroring
  /// --muted from globals.css (not something Flutter's ColorScheme exposes
  /// directly).
  static Color mutedOf(BuildContext context) {
    return Theme.of(context).brightness == Brightness.dark ? darkMuted : lightMuted;
  }
}

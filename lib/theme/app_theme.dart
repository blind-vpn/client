import 'package:flutter/material.dart';

class AppColors {
  // Background
  static const bg = Color(0xFF0D0D0F);
  static const bgCard = Color(0xFF1A1A1F);
  static const bgCardHover = Color(0xFF222228);
  static const bgElevated = Color(0xFF2A2A32);

  // Text
  static const textPrimary = Color(0xFFF5F5F7);
  static const textSecondary = Color(0xFF8E8E93);
  static const textMuted = Color(0xFF5A5A5E);

  // Accent
  static const accent = Color(0xFF3B82F6);
  static const accentDim = Color(0xFF1D4ED8);

  // Status
  static const connected = Color(0xFF22C55E);
  static const connectedDim = Color(0xFF16A34A);
  static const disconnected = Color(0xFF6B7280);
  static const error = Color(0xFFEF4444);

  // Border
  static const border = Color(0xFF2A2A32);
  static const borderLight = Color(0xFF3A3A42);
}

class AppTheme {
  static ThemeData get dark {
    return ThemeData(
      brightness: Brightness.dark,
      scaffoldBackgroundColor: AppColors.bg,
      fontFamily: 'Inter',
      colorScheme: const ColorScheme.dark(
        primary: AppColors.accent,
        surface: AppColors.bgCard,
        error: AppColors.error,
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: AppColors.bg,
        elevation: 0,
        titleTextStyle: TextStyle(
          fontFamily: 'Inter',
          fontSize: 18,
          fontWeight: FontWeight.w600,
          color: AppColors.textPrimary,
        ),
      ),
      textTheme: const TextTheme(
        headlineLarge: TextStyle(fontSize: 28, fontWeight: FontWeight.w700, color: AppColors.textPrimary),
        headlineMedium: TextStyle(fontSize: 22, fontWeight: FontWeight.w600, color: AppColors.textPrimary),
        titleMedium: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: AppColors.textPrimary),
        bodyLarge: TextStyle(fontSize: 16, fontWeight: FontWeight.w400, color: AppColors.textPrimary),
        bodyMedium: TextStyle(fontSize: 14, fontWeight: FontWeight.w400, color: AppColors.textSecondary),
        bodySmall: TextStyle(fontSize: 12, fontWeight: FontWeight.w400, color: AppColors.textMuted),
      ),
    );
  }
}

import 'package:cutable_mobile/core/theme/app_theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('light theme uses the cream background and pink accent', () {
    final theme = AppTheme.light();
    expect(theme.brightness, Brightness.light);
    expect(theme.scaffoldBackgroundColor, AppTheme.lightBackground);
    expect(theme.colorScheme.primary, AppTheme.accentPink);
  });

  test('dark theme mirrors globals.css --background/--accent', () {
    final theme = AppTheme.dark();
    expect(theme.brightness, Brightness.dark);
    expect(theme.scaffoldBackgroundColor, AppTheme.darkBackground);
    expect(theme.colorScheme.primary, AppTheme.darkAccent);
  });
}

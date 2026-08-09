import 'package:flutter/material.dart';

/// Mirrors apps/frontend/app/components/Brand.tsx.
class BrandMark extends StatelessWidget {
  const BrandMark({super.key, this.size = 40});

  final double size;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Image.asset('assets/brand/cutable-v3-transparent.png', height: size),
        const SizedBox(width: 10),
        Text(
          'Cutable',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.w600,
                letterSpacing: -0.02,
              ),
        ),
      ],
    );
  }
}

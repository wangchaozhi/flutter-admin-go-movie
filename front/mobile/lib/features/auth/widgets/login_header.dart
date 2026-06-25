import 'package:flutter/material.dart';

class LoginHeader extends StatelessWidget {
  const LoginHeader({super.key});

  @override
  Widget build(BuildContext context) {
    final textTheme = Theme.of(context).textTheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 52,
          height: 52,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(8),
            gradient: const LinearGradient(
              colors: [Color(0xFFF7C948), Color(0xFF25D0AB)],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            boxShadow: const [
              BoxShadow(
                color: Color(0x3325D0AB),
                blurRadius: 18,
                offset: Offset(0, 10),
              ),
            ],
          ),
          child: const Icon(
            Icons.movie_creation_outlined,
            color: Color(0xFF101318),
            size: 28,
          ),
        ),
        const SizedBox(height: 18),
        Text(
          'Go Movie',
          style: textTheme.headlineSmall?.copyWith(
            color: Colors.white,
            fontWeight: FontWeight.w900,
          ),
        ),
        const SizedBox(height: 6),
        Text(
          '登录继续观看，收藏与会员权益会自动同步。',
          style: textTheme.bodyMedium?.copyWith(
            color: const Color(0xFF9CA3AF),
            height: 1.45,
          ),
        ),
      ],
    );
  }
}

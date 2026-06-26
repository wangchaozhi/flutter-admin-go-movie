import 'package:flutter/material.dart';

class LoginHeader extends StatelessWidget {
  const LoginHeader({super.key});

  @override
  Widget build(BuildContext context) {
    final textTheme = Theme.of(context).textTheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.center,
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
        const SizedBox(height: 16),
        Text(
          'Go Movie',
          textAlign: TextAlign.center,
          style: textTheme.headlineSmall?.copyWith(
            color: Colors.white,
            fontWeight: FontWeight.w900,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          '登录后继续观看，收藏、进度和会员权益都会同步。',
          textAlign: TextAlign.center,
          style: textTheme.bodyMedium?.copyWith(
            color: const Color(0xFF9CA3AF),
            height: 1.5,
          ),
        ),
      ],
    );
  }
}

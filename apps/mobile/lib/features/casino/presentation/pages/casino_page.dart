import 'package:cached_network_image/cached_network_image.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/network/api_client.dart';

/// Casino lobby page.
///
/// Reads the catalog from the casino-service REST API
/// (`GET /api/v1/casino/games`, `GET /api/v1/casino/providers`) and keeps
/// all filtering client-side so the UI stays usable even when the backend
/// ignores unknown query parameters. Game launch goes through
/// `POST /api/v1/casino/games/launch`, which returns a one-time session
/// token — the user identity is taken from the JWT by the backend.
class CasinoPage extends StatefulWidget {
  const CasinoPage({super.key});

  @override
  State<CasinoPage> createState() => _CasinoPageState();
}

class _Game {
  final String id;
  final String name;
  final String providerId;
  final String providerName;
  final String category;
  final String imageUrl;
  final bool demoAvailable;
  final String rtp;

  const _Game({
    required this.id,
    required this.name,
    required this.providerId,
    required this.providerName,
    required this.category,
    required this.imageUrl,
    required this.demoAvailable,
    required this.rtp,
  });

  factory _Game.fromJson(Map<String, dynamic> json) {
    String str(List<String> keys) {
      for (final key in keys) {
        final value = json[key];
        if (value != null && '$value'.isNotEmpty) {
          return '$value';
        }
      }
      return '';
    }

    final rawDemo = json['is_demo_available'];
    return _Game(
      id: str(const ['id', 'game_id', 'slug']),
      name: str(const ['name', 'title']),
      providerId: str(const ['provider_id']),
      providerName: str(const ['provider_name', 'provider']),
      category: str(const ['category']),
      imageUrl: str(const ['thumbnail_url', 'image_url']),
      demoAvailable: rawDemo is bool ? rawDemo : rawDemo != null,
      rtp: str(const ['rtp']),
    );
  }
}

class _Provider {
  final String id;
  final String name;

  const _Provider({required this.id, required this.name});

  factory _Provider.fromJson(Map<String, dynamic> json) {
    return _Provider(
      id: '${json['id'] ?? ''}',
      name: '${json['name'] ?? ''}',
    );
  }
}

class _CasinoPageState extends State<CasinoPage> {
  final TextEditingController _search = TextEditingController();

  List<_Game> _games = const [];
  List<_Provider> _providers = const [];
  String _category = 'all';
  String _providerId = 'all';
  String _query = '';
  bool _loading = true;
  String? _error;
  final Set<String> _launching = <String>{};

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  List<dynamic> _asList(dynamic data) {
    if (data is List) {
      return data;
    }
    if (data is Map<String, dynamic>) {
      for (final key in const ['data', 'games', 'items']) {
        final nested = data[key];
        if (nested is List) {
          return nested;
        }
        if (nested is Map<String, dynamic>) {
          for (final inner in const ['games', 'items', 'data']) {
            if (nested[inner] is List) {
              return nested[inner] as List;
            }
          }
        }
      }
    }
    return const [];
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = getIt<ApiClient>();
      final results = await Future.wait([
        api.get('/api/v1/casino/games'),
        api.get('/api/v1/casino/providers'),
      ]);
      final games = _asList(results[0].data)
          .whereType<Map<String, dynamic>>()
          .map(_Game.fromJson)
          .where((game) => game.id.isNotEmpty && game.name.isNotEmpty)
          .toList();
      final providers = _asList(results[1].data)
          .whereType<Map<String, dynamic>>()
          .map(_Provider.fromJson)
          .where((provider) => provider.id.isNotEmpty)
          .toList();
      if (!mounted) {
        return;
      }
      setState(() {
        _games = games;
        _providers = providers;
        _loading = false;
      });
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = _message(e);
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  List<String> get _categories {
    final set = <String>{};
    for (final game in _games) {
      if (game.category.isNotEmpty) {
        set.add(game.category);
      }
    }
    final sorted = set.toList()..sort();
    return ['all', ...sorted];
  }

  List<_Game> get _filtered {
    final query = _query.trim().toLowerCase();
    return _games.where((game) {
      if (_category != 'all' && game.category != _category) {
        return false;
      }
      if (_providerId != 'all' && game.providerId != _providerId) {
        return false;
      }
      if (query.isNotEmpty) {
        final haystack = '${game.name} ${game.providerName}'.toLowerCase();
        if (!haystack.contains(query)) {
          return false;
        }
      }
      return true;
    }).toList();
  }

  Future<void> _launch(_Game game, {required bool demo}) async {
    if (_launching.contains(game.id)) {
      return;
    }
    setState(() {
      _launching.add(game.id);
    });
    try {
      final response = await getIt<ApiClient>().post(
        '/api/v1/casino/games/launch',
        data: {
          'game_id': game.id,
          'device_type': 'mobile',
          if (demo) 'mode': 'demo',
        },
      );
      if (!mounted) {
        return;
      }
      final body = response.data;
      final session =
          body is Map<String, dynamic> ? (body['data'] ?? body) : null;
      final launchUrl =
          session is Map ? '${session['launch_url'] ?? ''}' : '';
      final sessionId = session is Map ? '${session['id'] ?? ''}' : '';
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: Text(demo ? 'Demo session' : 'Game session'),
          content: Text(
            launchUrl.isEmpty
                ? 'Session ${sessionId.isEmpty ? 'created' : sessionId} '
                    'for "${game.name}". Open the launch URL from the '
                    'session details to play.'
                : 'Session ${sessionId.isEmpty ? 'created' : sessionId} '
                    'for "${game.name}".\n\n$launchUrl',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Close'),
            ),
          ],
        ),
      );
    } on DioException catch (e) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_message(e))),
      );
    } finally {
      if (mounted) {
        setState(() {
          _launching.remove(game.id);
        });
      }
    }
  }

  String _message(DioException e) {
    final data = e.response?.data;
    if (data is Map) {
      final err = data['error'];
      if (err is Map && err['message'] is String) {
        return err['message'] as String;
      }
      if (data['message'] is String) {
        return data['message'] as String;
      }
    }
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return 'Request timed out. Check your connection and retry.';
      case DioExceptionType.connectionError:
        return 'No connection to the server.';
      default:
        return e.message ?? 'Failed to load the casino catalog.';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Casino'),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? _ErrorView(message: _error!, onRetry: _load)
              : RefreshIndicator(
                  onRefresh: _load,
                  child: _CatalogView(
                    search: _search,
                    query: _query,
                    onQueryChanged: (value) =>
                        setState(() => _query = value),
                    categories: _categories,
                    category: _category,
                    onCategoryChanged: (value) =>
                        setState(() => _category = value),
                    providers: _providers,
                    providerId: _providerId,
                    onProviderChanged: (value) =>
                        setState(() => _providerId = value ?? 'all'),
                    games: _filtered,
                    launching: _launching,
                    onPlay: (game) => _launch(game, demo: false),
                    onDemo: (game) => _launch(game, demo: true),
                  ),
                ),
    );
  }
}

class _CatalogView extends StatelessWidget {
  final TextEditingController search;
  final String query;
  final ValueChanged<String> onQueryChanged;
  final List<String> categories;
  final String category;
  final ValueChanged<String> onCategoryChanged;
  final List<_Provider> providers;
  final String providerId;
  final ValueChanged<String?> onProviderChanged;
  final List<_Game> games;
  final Set<String> launching;
  final ValueChanged<_Game> onPlay;
  final ValueChanged<_Game> onDemo;

  const _CatalogView({
    required this.search,
    required this.query,
    required this.onQueryChanged,
    required this.categories,
    required this.category,
    required this.onCategoryChanged,
    required this.providers,
    required this.providerId,
    required this.onProviderChanged,
    required this.games,
    required this.launching,
    required this.onPlay,
    required this.onDemo,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = (constraints.maxWidth ~/ 180).clamp(2, 4);
        return CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
                child: TextField(
                  controller: search,
                  onChanged: onQueryChanged,
                  textInputAction: TextInputAction.search,
                  decoration: const InputDecoration(
                    hintText: 'Search games or providers',
                    prefixIcon: Icon(Icons.search),
                    border: OutlineInputBorder(),
                  ),
                ),
              ),
            ),
            SliverToBoxAdapter(
              child: SizedBox(
                height: 48,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  itemCount: categories.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (context, index) {
                    final value = categories[index];
                    final label =
                        value == 'all' ? 'All' : value.toUpperCase();
                    return ChoiceChip(
                      label: Text(label),
                      selected: category == value,
                      onSelected: (_) => onCategoryChanged(value),
                    );
                  },
                ),
              ),
            ),
            if (providers.isNotEmpty)
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
                  child: DropdownButtonFormField<String>(
                    value: providerId,
                    decoration: const InputDecoration(
                      labelText: 'Provider',
                      border: OutlineInputBorder(),
                    ),
                    items: [
                      const DropdownMenuItem(
                        value: 'all',
                        child: Text('All providers'),
                      ),
                      for (final provider in providers)
                        DropdownMenuItem(
                          value: provider.id,
                          child: Text(provider.name),
                        ),
                    ],
                    onChanged: onProviderChanged,
                  ),
                ),
              ),
            if (games.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(
                  child: Text('No games match the current filters.'),
                ),
              )
            else
              SliverPadding(
                padding: const EdgeInsets.all(12),
                sliver: SliverGrid(
                  delegate: SliverChildBuilderDelegate(
                    (context, index) => _GameCard(
                      game: games[index],
                      busy: launching.contains(games[index].id),
                      onPlay: () => onPlay(games[index]),
                      onDemo: () => onDemo(games[index]),
                    ),
                    childCount: games.length,
                  ),
                  gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: columns,
                    mainAxisSpacing: 12,
                    crossAxisSpacing: 12,
                    mainAxisExtent: 248,
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}

class _GameCard extends StatelessWidget {
  final _Game game;
  final bool busy;
  final VoidCallback onPlay;
  final VoidCallback onDemo;

  const _GameCard({
    required this.game,
    required this.busy,
    required this.onPlay,
    required this.onDemo,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: game.imageUrl.isEmpty
                ? const Icon(Icons.casino, size: 48)
                : CachedNetworkImage(
                    imageUrl: game.imageUrl,
                    fit: BoxFit.cover,
                    placeholder: (context, url) => const Center(
                      child: CircularProgressIndicator(),
                    ),
                    errorWidget: (context, url, error) =>
                        const Icon(Icons.casino, size: 48),
                  ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(8, 8, 8, 4),
            child: Text(
              game.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.titleSmall,
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Text(
              game.providerName.isEmpty
                  ? game.category
                  : '${game.providerName}'
                      '${game.category.isEmpty ? '' : ' · ${game.category}'}'
                      '${game.rtp.isEmpty ? '' : ' · RTP ${game.rtp}'}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(8),
            child: Row(
              children: [
                Expanded(
                  child: ElevatedButton(
                    onPressed: busy ? null : onPlay,
                    child: busy
                        ? const SizedBox(
                            height: 18,
                            width: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Play'),
                  ),
                ),
                if (game.demoAvailable) ...[
                  const SizedBox(width: 8),
                  Expanded(
                    child: OutlinedButton(
                      onPressed: busy ? null : onDemo,
                      child: const Text('Demo'),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorView({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 48),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: onRetry,
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}

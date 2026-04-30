import 'package:flutter/material.dart';

class SportsbookScreen extends StatefulWidget {
  const SportsbookScreen({super.key});

  @override
  State<SportsbookScreen> createState() => _SportsbookScreenState();
}

class _SportsbookScreenState extends State<SportsbookScreen> {
  bool _showLiveOnly = false;
  String _selectedSport = 'all';

  final List<Map<String, dynamic>> _mockEvents = [
    {
      'id': '1',
      'sport': 'Football',
      'league': 'Premier League',
      'homeTeam': 'Arsenal',
      'awayTeam': 'Liverpool',
      'startTime': DateTime.now().add(const Duration(hours: 1)),
      'isLive': true,
      'odds': {'home': 2.50, 'draw': 3.20, 'away': 2.80},
    },
    {
      'id': '2',
      'sport': 'Football',
      'league': 'La Liga',
      'homeTeam': 'Real Madrid',
      'awayTeam': 'Barcelona',
      'startTime': DateTime.now().add(const Duration(hours: 5)),
      'isLive': false,
      'odds': {'home': 2.10, 'draw': 3.40, 'away': 3.20},
    },
    {
      'id': '3',
      'sport': 'Basketball',
      'league': 'NBA',
      'homeTeam': 'Lakers',
      'awayTeam': 'Celtics',
      'startTime': DateTime.now().add(const Duration(hours: 12)),
      'isLive': false,
      'odds': {'home': 1.85, 'away': 1.95},
    },
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sportsbook'),
        actions: [
          SwitchListTile(
            title: const Text('Live', style: TextStyle(fontSize: 12)),
            value: _showLiveOnly,
            onChanged: (value) => setState(() => _showLiveOnly = value),
            contentPadding: EdgeInsets.zero,
          ),
        ],
      ),
      body: Column(
        children: [
          // Sport filter
          SizedBox(
            height: 50,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              children: [
                _buildSportChip('All', '🏆'),
                _buildSportChip('Football', '⚽'),
                _buildSportChip('Basketball', '🏀'),
                _buildSportChip('Tennis', '🎾'),
                _buildSportChip('Hockey', '🏒'),
              ],
            ),
          ),
          const Divider(),
          // Events list
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.all(16),
              itemCount: _mockEvents.length,
              itemBuilder: (context, index) {
                final event = _mockEvents[index];
                return Card(
                  margin: const EdgeInsets.only(bottom: 12),
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Text(
                              event['league'],
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                            const Spacer(),
                            if (event['isLive'])
                              Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 8,
                                  vertical: 2,
                                ),
                                decoration: BoxDecoration(
                                  color: Colors.red.shade100,
                                  borderRadius: BorderRadius.circular(4),
                                ),
                                child: Text(
                                  'LIVE',
                                  style: TextStyle(
                                    color: Colors.red.shade700,
                                    fontSize: 10,
                                    fontWeight: FontWeight.bold,
                                  ),
                                ),
                              ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(
                          '${event['homeTeam']} vs ${event['awayTeam']}',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 12),
                        Row(
                          children: [
                            Expanded(
                              child: _buildOddsButton('1', event['odds']['home']),
                            ),
                            if (event['odds']['draw'] != null) ...[
                              const SizedBox(width: 8),
                              Expanded(
                                child: _buildOddsButton('X', event['odds']['draw']),
                              ),
                              const SizedBox(width: 8),
                            ],
                            Expanded(
                              child: _buildOddsButton('2', event['odds']['away']),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSportChip(String name, String icon) {
    final isSelected = _selectedSport == name ||
        (name == 'All' && _selectedSport == 'all');
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: FilterChip(
        label: Text('$icon $name'),
        selected: isSelected,
        onSelected: (selected) {
          setState(() {
            _selectedSport = name == 'All' ? 'all' : name.toLowerCase();
          });
        },
      ),
    );
  }

  Widget _buildOddsButton(String label, double odds) {
    return OutlinedButton(
      onPressed: () {
        // Add to bet slip
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Добавлено: $label с кэф. ${odds.toStringAsFixed(2)}')),
        );
      },
      child: Column(
        children: [
          Text(label, style: const TextStyle(fontSize: 12)),
          Text(
            odds.toStringAsFixed(2),
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
        ],
      ),
    );
  }
}


# analyze-pt-stalk

analyze-mysqladmin.py - it will turn collected MySQL variables values into nice graphical dashboards.
  
## how to run

install dependencies

```bash
pip install flask pandas plotly dash dash_bootstrap_components tqdm
```

run the script

```bash
python3 analyze-mysqladmin.py /pt-stalk/collected/
Processing files: 100%|███████████████████████████████████████████| 10/10 [00:00<00:00, 10.83it/s]
Dash is running on http://127.0.0.1:8050/

 * Serving Flask app 'test'
 * Debug mode: on

```

## features

now the tool supported many pt-stalk outputs

$ python3 analyze-mysqladmin.py /pt-stalk/collected/hostname1 /pt-stalk/collected/hostname2
Processing files: 100%|███████████████████████████████████████████| 10/10 [00:00<00:00, 10.83it/s]
Processing files: 100%|███████████████████████████████████████████| 10/10 [00:00<00:00, 10.83it/s]
Dash is running on http://127.0.0.1:8050/

 * Serving Flask app 'analyze-mysqladmin'
 * Debug mode: on

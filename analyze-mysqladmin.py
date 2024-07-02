import sys
import pandas as pd
import re
from flask import Flask
from dash import dcc, html
import dash
import dash_bootstrap_components as dbc
from dash.dependencies import Input, Output
import plotly.graph_objs as go

# Flask server
server = Flask(__name__)

# Dash app
app = dash.Dash(__name__, server=server, external_stylesheets=[dbc.themes.BOOTSTRAP])

def parse_file(file_path):
    with open(file_path, 'r') as file:
        file_contents = file.readlines()
    
    data = []
    pattern = re.compile(r'\|\s+(\w+)\s+\|\s+(\S+)\s+\|')
    
    for line in file_contents:
        match = pattern.match(line)
        if match:
            variable_name, value = match.groups()
            if value.isdigit():
                data.append((variable_name, int(value)))
    
    return pd.DataFrame(data, columns=['Variable_name', 'Value'])

def calculate_deltas(file_path):
    data_df = parse_file(file_path)
    data_df['Group'] = data_df['Variable_name'].apply(lambda x: x.split('_')[0])
    
    result = {}
    for group, group_df in data_df.groupby('Group'):
        group_result = {}
        for variable in group_df['Variable_name'].unique():
            variable_df = group_df[group_df['Variable_name'] == variable]
            values = variable_df['Value'].tolist()
            deltas = [0] + [values[i] - values[i-1] for i in range(1, len(values))]
            if any(delta != 0 for delta in deltas):  # Only keep if there are non-zero deltas
                group_result[variable] = deltas
        if group_result:  # Only keep if the group has non-zero deltas
            result[group] = group_result
    
    return result

deltas = calculate_deltas(sys.argv[1])

# Generate graphs
graph_list = []
for group, group_deltas in deltas.items():
    graph_list.append(
        dbc.Col(
            dbc.Card(
                dbc.CardBody([
                    html.H5(f'{group} Variables', className='card-title'),
                    dcc.Graph(
                        id=f'graph-{group}',
                        figure={
                            'data': [
                                go.Scatter(
                                    x=list(range(len(delta_values))),
                                    y=delta_values,
                                    mode='lines+markers',
                                    name=variable,
                                    hovertemplate=f'{variable}: %{{y}}<extra></extra>'
                                ) for variable, delta_values in group_deltas.items()
                            ]
                        }
                    )
                ])
            ),
            width=6,
            style={'margin-bottom': '20px'}
        )
    )

# Dash layout
app.layout = dbc.Container([
    dbc.Row([
        dbc.Col(html.H1("mysqladmin output", className='text-center mb-4'), width=12)
    ]),
    dbc.Row(graph_list)
], fluid=True, className='bg-light')

if __name__ == '__main__':
    if len(sys.argv) != 2:
        print("Usage: python app.py <path_to_mysqladmin_file>")
        sys.exit(1)
    
    app.run_server(debug=True)

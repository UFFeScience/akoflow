# Figure8: shared representative observations

Four physical executions represent six scheduler/environment cases. HPC HEFT and MOHEFT-TIME use the audited HEFT observation1608.7642674446106s. HPC+medium PRISM-TIME and HEFT use the audited PRISM observation3353.9606051445007s. These are shared observations of equivalent schedules, not two independent measurements with identical durations. Asterisks identify each pair within its panel.

Independent replays remain unchanged in independent-executions.csv/json: MOHEFT-TIME/HPC1261.642051935196s and HEFT/HPC+medium3173.3246052265167s. The other four independent observations are retained too. All forecasts and frozen input hashes are unchanged. Repeated measurements illustrate run-to-run variation; they do not isolate its cause or establish scheduling superiority.

The plotted CSV/JSON retain actual real_run_id, executed_plan_id and trace/request hashes, and explicitly label shared_real_observation, shared_real_group and figure_observation_basis. They also link each independent execution's ID and duration. No source observation was overwritten. Error and range values correspond to the representative real used by the figure.

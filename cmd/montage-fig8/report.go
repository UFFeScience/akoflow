package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type comparisonRow struct {
	Case, PlanID, SimID, RealID, Status  string
	Planned, Simulated, Real, Cost, Wall string
}

func metric(v any) string {
	n, ok := v.(float64)
	if !ok {
		return "—"
	}
	return strconv.FormatFloat(n, 'f', 2, 64)
}
func comparison(base, dir, campaignID string) error {
	rows := []comparisonRow{}
	client := apiClient{base}
	for _, kind := range []string{"hpc", "micro", "medium"} {
		r := comparisonRow{Case: kind, Status: "not submitted", Planned: "—", Simulated: "—", Real: "—", Cost: "—", Wall: "—", SimID: campaignID + "-" + kind + "-simulation", RealID: campaignID + "-" + kind + "-real"}
		b, e := os.ReadFile(filepath.Join(dir, kind+"-plan.json"))
		if e == nil {
			var p obj
			if e = json.Unmarshal(b, &p); e != nil {
				return e
			}
			r.PlanID, _ = p["id"].(string)
			predicted, _ := p["predicted"].(map[string]any)
			r.Planned = metric(predicted["makespanSeconds"])
		}
		for _, mode := range []string{"simulation", "real"} {
			id := r.SimID
			if mode == "real" {
				id = r.RealID
			}
			path := "/execution-runs/" + id + "/?view=status"
			if !client.exists(path) {
				continue
			}
			var result obj
			if e := client.call("GET", path, nil, &result); e != nil {
				return e
			}
			run, _ := result["run"].(map[string]any)
			status, _ := run["status"].(string)
			if mode == "simulation" {
				if status == "completed" {
					r.Simulated = metric(run["makespanSeconds"])
				}
			} else {
				r.Status = status
				if status == "completed" {
					r.Real = metric(run["makespanSeconds"])
					r.Cost = metric(run["cost"])
					start, _ := run["startedAt"].(string)
					finish, _ := run["finishedAt"].(string)
					a, e1 := time.Parse(time.RFC3339, start)
					z, e2 := time.Parse(time.RFC3339, finish)
					if e1 == nil && e2 == nil {
						r.Wall = metric(z.Sub(a).Seconds())
					}
				}
			}
			save(filepath.Join(dir, id+"-result.json"), result)
		}
		rows = append(rows, r)
	}
	save(filepath.Join(dir, "comparison.json"), rows)
	file, e := os.Create(filepath.Join(dir, "comparison.csv"))
	if e != nil {
		return e
	}
	w := csv.NewWriter(file)
	e = w.Write([]string{"case", "plan_id", "planned_seconds", "simulated_seconds", "real_seconds", "real_wall_seconds", "real_cost_estimate_usd", "real_status", "simulation_run_id", "real_run_id"})
	if e != nil {
		file.Close()
		return e
	}
	for _, r := range rows {
		if e = w.Write([]string{r.Case, r.PlanID, r.Planned, r.Simulated, r.Real, r.Wall, r.Cost, r.Status, r.SimID, r.RealID}); e != nil {
			file.Close()
			return e
		}
	}
	w.Flush()
	if e = w.Error(); e != nil {
		file.Close()
		return e
	}
	if e = file.Close(); e != nil {
		return e
	}
	const page = `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Montage 58 · HEFT · Figura 8</title><style>body{font:15px system-ui;margin:64px auto;max-width:1100px;color:#181818;background:#fafafa}h1{font-size:32px}p{color:#555;line-height:1.6}table{width:100%;border-collapse:collapse;background:white}th,td{text-align:right;padding:16px;border-bottom:1px solid #ddd}th:first-child,td:first-child{text-align:left}small{color:#777}code{font-size:12px}footer{margin-top:24px}</style><h1>Montage 58 · HEFT</h1><p>Figura 8: HPC (64 slots), HPC (1) + 10 micros (1 cada), HPC (1) + 2 mediums (2 cada). Disco padrão do PlaFRIM, com liberação após o consumo dos dados. O mesmo plano é usado na simulação e na execução real.</p><table><tr><th>Cenário</th><th>Planejado (s)</th><th>Simulado (s)</th><th>Real (s)</th><th>Parede real (s)</th><th>Custo real estimado (USD)</th><th>Estado real</th></tr>{{range .}}<tr><td>{{.Case}}</td><td>{{.Planned}}</td><td>{{.Simulated}}</td><td>{{.Real}}</td><td>{{.Wall}}</td><td>{{.Cost}}</td><td>{{.Status}}</td></tr>{{end}}</table><p>“—” significa resultado indisponível. O planejado usa HEFT; a simulação inclui os estágios modelados de preparação e transferência. Custo é uma estimativa do catálogo e do ciclo de vida, não a fatura do Google Cloud. O pré-check conservador de quota tem exceção autorizada por execução; a quota do sistema operacional continua ativa.</p><footer><small>Previsões congeladas antes das respectivas execuções. Perfis históricos e medições independentes de transferência têm limitações de amostra e domínio; consultar os snapshots e os arquivos de evidência junto deste relatório.</small></footer></html>`
	t, e := template.New("comparison").Parse(page)
	if e != nil {
		return e
	}
	f, e := os.Create(filepath.Join(dir, "comparison.html"))
	if e != nil {
		return e
	}
	defer f.Close()
	if e = t.Execute(f, rows); e != nil {
		return e
	}
	fmt.Println("Comparison:", filepath.Join(dir, "comparison.html"))
	return nil
}

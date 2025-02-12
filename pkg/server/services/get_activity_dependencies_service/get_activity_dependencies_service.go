package get_activity_dependencies_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

// GetActivityDependenciesService is a service that returns the dependencies of an activity.
type GetActivityDependenciesService struct {
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() GetActivityDependenciesService {
	return GetActivityDependenciesService{
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}

// GetActivityDependencies recupera todas as dependências de atividades para um dado fluxo de trabalho.
// Este método organiza e retorna um mapeamento onde cada chave é o ID de uma atividade e o valor é uma lista
// de atividades das quais ela depende, incluindo dependências diretas e indiretas.
//
// Parâmetros:
// - workflowId: O identificador inteiro do fluxo de trabalho cujas dependências de atividades estão sendo solicitadas.
//
// Retorna:
// - Um mapa (MapActivityDependencies) representando as dependências entre as atividades do fluxo de trabalho.
//
//	O processo envolve várias etapas:
//	1. Recuperar o fluxo de trabalho e suas atividades associadas usando os respectivos repositórios.
//	2. Mapear cada atividade pelo seu ID para facilitar o acesso durante o processamento de dependências.
//	3. Inicializar estruturas de dados para armazenar as dependências processadas e evitar duplicatas.
//	4. Preencher recursivamente as dependências de cada atividade usando fillActivityDependencies.
//	5. Converter o conjunto de dependências de cada atividade em uma lista e associá-la no mapa de retorno.
func (g *GetActivityDependenciesService) GetActivityDependencies(workflowId int) workflow_activity_entity.MapActivityDependencies {
	wf, _ := g.workflowRepository.Find(workflowId)
	wfa, _ := g.activityRepository.GetActivitiesByWorkflowIds([]int{workflowId})
	wfaDependencies, _ := g.activityRepository.GetWfaDependencies(workflowId)
	activityDependencies := make(workflow_activity_entity.MapActivityDependencies)
	setDependencies := make(map[int]map[int]workflow_activity_entity.WorkflowActivities)

	mapWfa := make(map[int]workflow_activity_entity.WorkflowActivities)
	for _, w := range wfa[wf.Id] {
		mapWfa[w.Id] = w
		activityDependencies[w.Id] = make([]workflow_activity_entity.WorkflowActivities, 0)
		setDependencies[w.Id] = make(map[int]workflow_activity_entity.WorkflowActivities)
	}

	for _, wfaDep := range wfaDependencies {
		for _, dep := range g.fillActivityDependencies(wfaDep.DependsOnId, mapWfa, wfaDependencies) {
			setDependencies[wfaDep.ActivityId][dep.Id] = dep
		}
		activityDependencies[wfaDep.ActivityId] = g.setDependenciesToArray(setDependencies[wfaDep.ActivityId])
	}

	return activityDependencies
}

func (g *GetActivityDependenciesService) GetActivityDependenciesByActivity(workflowId, activityId int) workflow_activity_entity.MapActivityDependencies {
	wfaDependencies, _ := g.activityRepository.GetWfaDependencies(workflowId)
	mapWfa := make(map[int]workflow_activity_entity.WorkflowActivities)

	wfa, _ := g.activityRepository.GetActivitiesByWorkflowIds([]int{workflowId})

	for _, w := range wfa[workflowId] {
		mapWfa[w.Id] = w
	}

	returns := make(workflow_activity_entity.MapActivityDependencies)
	for _, wfaDep := range wfaDependencies {
		if wfaDep.ActivityId == activityId {
			returns[wfaDep.ActivityId] = append(returns[wfaDep.ActivityId], mapWfa[wfaDep.DependsOnId])
		}
	}

	return returns

}

func (g *GetActivityDependenciesService) GetActivityDependenciesByWorkflow(workflowId int) workflow_activity_entity.MapActivityDependencies {
	wfaDependencies, _ := g.activityRepository.GetWfaDependencies(workflowId)
	mapWfa := make(map[int]workflow_activity_entity.WorkflowActivities)

	wfa, _ := g.activityRepository.GetActivitiesByWorkflowIds([]int{workflowId})

	for _, w := range wfa[workflowId] {
		mapWfa[w.Id] = w
	}

	returns := make(workflow_activity_entity.MapActivityDependencies)
	for _, wfaDep := range wfaDependencies {
		returns[wfaDep.ActivityId] = append(returns[wfaDep.ActivityId], mapWfa[wfaDep.DependsOnId])
	}

	return returns

}

// fillActivityDependencies é uma função recursiva que preenche as dependências de uma atividade específica.
// Este método é crítico para a funcionalidade do GetActivityDependencies, permitindo a resolução de dependências
// tanto diretas quanto indiretas de uma atividade.
//
// Parâmetros:
// - dependWfa: O ID da atividade cujas dependências estão sendo preenchidas.
// - mapWfa: Um mapa de todas as atividades disponíveis, facilitando o acesso durante a recursão.
// - wfaDependencies: Uma lista de todas as dependências conhecidas entre atividades no fluxo de trabalho.
//
// Retorna:
// - Uma lista de atividades que representam todas as dependências diretas e indiretas da atividade especificada.
//
// O método funciona seguindo estes passos:
//  1. Verificar se a atividade especificada está presente no mapa de atividades; se sim, adicioná-la ao conjunto de dependências.
//  2. Iterar sobre todas as dependências conhecidas, e para cada uma que corresponda à atividade em questão,
//     chamar fillActivityDependencies recursivamente para resolver suas dependências.
//  3. Converter o conjunto de dependências coletadas em uma lista para ser retornada.
func (g *GetActivityDependenciesService) fillActivityDependencies(dependWfa int, mapWfa map[int]workflow_activity_entity.WorkflowActivities, wfaDependencies []workflow_activity_entity.WorkflowActivityDependencyDatabase) []workflow_activity_entity.WorkflowActivities {
	setDependencies := make(map[int]workflow_activity_entity.WorkflowActivities)

	if wfa, ok := mapWfa[dependWfa]; ok {
		setDependencies[wfa.Id] = wfa
		for _, wfaDep := range wfaDependencies {
			if wfaDep.ActivityId == dependWfa {
				for _, dep := range g.fillActivityDependencies(wfaDep.DependsOnId, mapWfa, wfaDependencies) {
					setDependencies[dep.Id] = dep
				}
			}
		}
	}
	return g.setDependenciesToArray(setDependencies)
}

// setDependenciesToArray converte um mapa de dependências de atividades em uma lista ordenada.
// Este método é utilizado para transformar o conjunto de dependências, armazenadas como um mapa para evitar duplicatas,
// em uma lista ordenada de atividades por seu ID. Isso facilita a manipulação subsequente das dependências,
// como iterá-las em ordem ou apresentá-las de forma sequencial.
//
// Parâmetros:
//   - setDependencies: Um mapa onde cada chave é o ID de uma atividade e o valor é o objeto da atividade correspondente.
//     Este mapa representa o conjunto de todas as dependências de uma atividade específica.
//
// Retorna:
// - Uma lista de objetos WorkflowActivities representando as dependências da atividade, ordenadas pelo ID da atividade.
//
// O método executa os seguintes passos:
//  1. Inicializa uma lista vazia `dependencies` para coletar os objetos de atividade do mapa.
//  2. Itera sobre o mapa de dependências, adicionando cada objeto de atividade à lista `dependencies`.
//  3. Inicializa uma nova lista `sorted` para armazenar as atividades ordenadas.
//  4. Realiza uma ordenação simples das atividades na lista `dependencies` com base em seus IDs,
//     utilizando um algoritmo de ordenação por seleção.
//     - Durante a ordenação, as atividades são comparadas pelos seus IDs, e a ordem na lista é ajustada conforme necessário.
//  5. A cada iteração do processo de ordenação, a atividade correntemente ordenada é adicionada à lista `sorted`.
//  6. Retorna a lista `sorted` contendo todas as dependências ordenadas por ID.
//
// Nota: Este método assume que todos os IDs de atividade são únicos e utiliza uma ordenação simples, que é eficaz para
// conjuntos de dados pequenos a moderados.
func (g *GetActivityDependenciesService) setDependenciesToArray(setDependencies map[int]workflow_activity_entity.WorkflowActivities) []workflow_activity_entity.WorkflowActivities {
	dependencies := make([]workflow_activity_entity.WorkflowActivities, 0)
	for _, dep := range setDependencies {
		dependencies = append(dependencies, dep)
	}

	sorted := make([]workflow_activity_entity.WorkflowActivities, 0)

	for i := 0; i < len(dependencies); i++ {
		for j := i + 1; j < len(dependencies); j++ {
			if dependencies[i].Id > dependencies[j].Id {
				dependencies[i], dependencies[j] = dependencies[j], dependencies[i]
			}
		}
		sorted = append(sorted, dependencies[i])

	}
	return sorted

}

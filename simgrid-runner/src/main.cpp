#include <simgrid/s4u.hpp>

#include <algorithm>
#include <fstream>
#include <iostream>
#include <map>
#include <set>
#include <stdexcept>
#include <string>
#include <vector>

#include <nlohmann/json.hpp>

namespace sg4 = simgrid::s4u;
using json    = nlohmann::json;
static int mailbox_token = 1;

struct Arguments {
  std::string platform;
  std::string input;
  std::string output;
};

struct TaskModel {
  std::string id;
  std::string assignment_id;
  std::string resource_id;
  double price_per_second = 0;
  double flops            = 0;
  double overhead_seconds = 0;
  double started_at       = 0;
  double compute_started_at = 0;
  double compute_finished_at = 0;
  double finished_at      = 0;
};

struct TransferModel {
  std::string producer_id;
  std::string consumer_id;
  std::string source_resource_id;
  std::string target_resource_id;
  long long bytes       = 0;
  double price_per_byte = 0;
  double started_at     = 0;
  double finished_at    = 0;
};

static Arguments parse_arguments(int argc, char** argv)
{
  Arguments arguments;
  for (int index = 1; index + 1 < argc; index += 2) {
    const std::string option = argv[index];
    if (option == "--platform")
      arguments.platform = argv[index + 1];
    else if (option == "--input")
      arguments.input = argv[index + 1];
    else if (option == "--output")
      arguments.output = argv[index + 1];
    else
      throw std::runtime_error("unknown argument: " + option);
  }
  if (arguments.platform.empty() || arguments.input.empty() || arguments.output.empty())
    throw std::runtime_error("usage: akoflow-simgrid-runner --platform FILE --input FILE --output FILE");
  return arguments;
}

static json read_json(const std::string& path)
{
  std::ifstream stream(path);
  if (!stream)
    throw std::runtime_error("cannot open input file: " + path);
  json document;
  stream >> document;
  return document;
}

static void write_json(const std::string& path, const json& document)
{
  std::ofstream stream(path);
  if (!stream)
    throw std::runtime_error("cannot open output file: " + path);
  stream << document.dump(2) << '\n';
}

static std::map<std::string, TaskModel> create_tasks(sg4::Engine& engine, const json& input)
{
  std::map<std::string, TaskModel> tasks;
  for (const auto& value : input.at("tasks")) {
    TaskModel task;
    task.id               = value.at("id").get<std::string>();
    task.assignment_id    = value.at("assignmentId").get<std::string>();
    task.resource_id      = value.at("resourceId").get<std::string>();
    task.price_per_second = value.value("pricePerSecond", 0.0);
    engine.host_by_name(task.resource_id);
    task.flops = std::max(0.0, value.at("flops").get<double>());
    task.overhead_seconds = std::max(0.0, value.value("overheadSeconds", 0.0));
    if (!tasks.emplace(task.id, task).second)
      throw std::runtime_error("duplicate task: " + task.id);
  }
  return tasks;
}

static std::vector<TransferModel> create_transfers(const json& input, const std::map<std::string, TaskModel>& tasks)
{
  std::vector<TransferModel> transfers;
  for (const auto& value : input.at("dependencies")) {
    const std::string producer_id = value.at("producerId").get<std::string>();
    const std::string consumer_id = value.at("consumerId").get<std::string>();
    auto& producer                = tasks.at(producer_id);
    auto& consumer                = tasks.at(consumer_id);
    const long long bytes         = value.value("bytes", 0LL);
    if (bytes <= 0 || producer.resource_id == consumer.resource_id)
      continue;
    TransferModel transfer;
    transfer.producer_id       = producer_id;
    transfer.consumer_id       = consumer_id;
    transfer.source_resource_id = producer.resource_id;
    transfer.target_resource_id = consumer.resource_id;
    transfer.bytes              = bytes;
    transfer.price_per_byte     = value.value("pricePerByte", 0.0);
    transfers.push_back(transfer);
  }
  return transfers;
}

static std::string mailbox_name(const std::string& run_id, const std::string& kind,
                                const std::string& predecessor, const std::string& successor)
{
  return run_id + ":" + kind + ":" + predecessor + ":" + successor;
}

static void run_simulation(sg4::Engine& engine, const json& input,
                           std::map<std::string, TaskModel>& tasks,
                           std::vector<TransferModel>& transfers)
{
  const auto run_id = input.at("runId").get<std::string>();
  std::map<std::string, std::vector<std::pair<std::string, std::string>>> incoming_dependencies;
  std::map<std::string, std::vector<std::string>> outgoing_dependency_signals;
  std::map<std::string, std::vector<std::string>> incoming_lanes;
  std::map<std::string, std::vector<std::string>> outgoing_lanes;

  for (const auto& dependency : input.at("dependencies")) {
    const auto producer = dependency.at("producerId").get<std::string>();
    const auto consumer = dependency.at("consumerId").get<std::string>();
    incoming_dependencies[consumer].push_back({producer, mailbox_name(run_id, "data", producer, consumer)});
    outgoing_dependency_signals[producer].push_back(mailbox_name(run_id, "ready", producer, consumer));
  }
  for (const auto& order : input.at("resourceLaneOrders")) {
    const auto predecessor = order.at("predecessorId").get<std::string>();
    const auto successor   = order.at("successorId").get<std::string>();
    const auto mailbox     = mailbox_name(run_id, "lane", predecessor, successor);
    incoming_lanes[successor].push_back(mailbox);
    outgoing_lanes[predecessor].push_back(mailbox);
  }

  for (auto& transfer : transfers) {
    auto* transfer_model = &transfer;
    auto* source_host = engine.host_by_name(transfer.source_resource_id);
    sg4::Actor::create("transfer-" + transfer.producer_id + "-" + transfer.consumer_id, source_host,
                       [transfer_model, run_id]() {
      sg4::Mailbox::by_name(mailbox_name(run_id, "ready", transfer_model->producer_id,
                                         transfer_model->consumer_id))->get<void>();
      transfer_model->started_at = sg4::Engine::get_clock();
      sg4::Mailbox::by_name(mailbox_name(run_id, "data", transfer_model->producer_id,
                                         transfer_model->consumer_id))
          ->put(&mailbox_token, transfer_model->bytes);
      transfer_model->finished_at = sg4::Engine::get_clock();
    });
  }

  for (auto& [id, task] : tasks) {
    auto* task_model = &task;
    auto* host = engine.host_by_name(task.resource_id);
    sg4::Actor::create("task-" + id, host,
                       [task_model, id, &incoming_dependencies, &outgoing_dependency_signals,
                        &incoming_lanes, &outgoing_lanes, &tasks, run_id]() {
      for (const auto& [producer_id, mailbox] : incoming_dependencies[id]) {
        if (tasks.at(producer_id).resource_id == task_model->resource_id)
          sg4::Mailbox::by_name(mailbox_name(run_id, "ready", producer_id, id))->get<void>();
        else
          sg4::Mailbox::by_name(mailbox)->get<void>();
      }
      for (const auto& mailbox : incoming_lanes[id])
        sg4::Mailbox::by_name(mailbox)->get<void>();
      task_model->started_at = sg4::Engine::get_clock();
      if (task_model->overhead_seconds > 0)
        sg4::this_actor::sleep_for(task_model->overhead_seconds);
      task_model->compute_started_at = sg4::Engine::get_clock();
      sg4::this_actor::execute(task_model->flops);
      task_model->compute_finished_at = sg4::Engine::get_clock();
      task_model->finished_at = sg4::Engine::get_clock();
      for (const auto& mailbox : outgoing_dependency_signals[id])
		sg4::Mailbox::by_name(mailbox)->put_init(&mailbox_token, 0)->detach();
      for (const auto& mailbox : outgoing_lanes[id])
		sg4::Mailbox::by_name(mailbox)->put_init(&mailbox_token, 0)->detach();
    });
  }
  engine.run();
}

static std::map<std::string, double> task_costs(const std::map<std::string, TaskModel>& tasks,
                                                const std::map<std::string, double>& effective_starts)
{
  struct Window {
    double start  = 0;
    double finish = 0;
    double price  = 0;
    std::vector<std::string> task_ids;
  };
  std::map<std::string, Window> windows;
  for (const auto& [id, task] : tasks) {
    auto& window = windows[task.resource_id];
    const double start = effective_starts.at(id);
    const double finish = task.finished_at;
    if (window.task_ids.empty()) {
      window.start = start;
      window.finish = finish;
    } else {
      window.start = std::min(window.start, start);
      window.finish = std::max(window.finish, finish);
    }
    window.price = task.price_per_second;
    window.task_ids.push_back(id);
  }
  std::map<std::string, double> costs;
  for (const auto& [_, window] : windows) {
    const double total = std::max(0.0, window.finish - window.start) * window.price;
    double duration_sum = 0;
    for (const auto& id : window.task_ids)
      duration_sum += tasks.at(id).finished_at - effective_starts.at(id);
    for (const auto& id : window.task_ids) {
      const auto& task = tasks.at(id);
      const double duration = task.finished_at - effective_starts.at(id);
      costs[id] = total * (duration_sum > 0 ? duration / duration_sum : 1.0 / window.task_ids.size());
    }
  }
  return costs;
}

static json build_result(const json& input, const std::map<std::string, TaskModel>& tasks,
                         const std::vector<TransferModel>& transfers)
{
  json result = {
      {"runId", input.at("runId")}, {"planId", input.at("planId")}, {"mode", "simulation"},
      {"tasks", json::array()}, {"transfers", json::array()},
  };
  double makespan = 0;
  double compute = 0;
  double queue = 0;
  double transfer_seconds = 0;
  double overhead_seconds = 0;
  double total_cost = 0;
  std::map<std::string, double> data_ready;
  std::map<std::string, double> inbound_transfer;
  for (const auto& dependency : input.at("dependencies")) {
    const auto producer = dependency.at("producerId").get<std::string>();
    const auto consumer = dependency.at("consumerId").get<std::string>();
    data_ready[consumer] = std::max(data_ready[consumer], tasks.at(producer).finished_at);
  }
  for (const auto& transfer : transfers) {
    data_ready[transfer.consumer_id] = std::max(data_ready[transfer.consumer_id], transfer.finished_at);
    inbound_transfer[transfer.consumer_id] += std::max(0.0, transfer.finished_at - transfer.started_at);
  }
  auto execution_ready = data_ready;
  for (const auto& order : input.at("resourceLaneOrders")) {
    const auto predecessor = order.at("predecessorId").get<std::string>();
    const auto successor   = order.at("successorId").get<std::string>();
    execution_ready[successor] = std::max(execution_ready[successor], tasks.at(predecessor).finished_at);
  }
  // SimGrid fires Exec's start callback when an activity enters its started
  // lifecycle, including while it is vetoed by an unfinished predecessor.  The
  // execution trace exposed by AkôFlow must instead report when computation can
  // effectively begin, after all required data has arrived.
  std::map<std::string, double> effective_starts;
  for (const auto& [id, task] : tasks)
    effective_starts[id] = std::max(task.started_at, execution_ready[id]);
  const auto costs = task_costs(tasks, effective_starts);
  for (const auto& [id, task] : tasks) {
    const double start = effective_starts.at(id);
    const double finish = task.finished_at;
    const double duration = std::max(0.0, task.compute_finished_at - task.compute_started_at);
    makespan = std::max(makespan, finish);
    compute += duration;
    const double ready = data_ready[id];
    const double queued = std::max(0.0, start - ready);
    queue += queued;
    overhead_seconds += task.overhead_seconds;
    total_cost += costs.at(id);
    result["tasks"].push_back({
        {"id", input.at("runId").get<std::string>() + ":" + id},
        {"executionRunId", input.at("runId")}, {"planAssignmentId", task.assignment_id},
        {"activityId", id}, {"plannedResourceId", task.resource_id}, {"allocatedResourceId", task.resource_id},
        {"attempt", 1}, {"status", "completed"}, {"readyAt", ready}, {"dataReadyAt", ready},
        {"queuedAt", ready}, {"startedAt", start}, {"finishedAt", finish},
        {"runtimeSeconds", duration}, {"queueSeconds", queued}, {"transferSeconds", inbound_transfer[id]},
        {"interferenceSeconds", 0}, {"overheadSeconds", task.overhead_seconds}, {"cost", costs.at(id)},
    });
  }
  for (const auto& transfer : transfers) {
    const double start = transfer.started_at;
    const double finish = transfer.finished_at;
    const double duration = std::max(0.0, finish - start);
    const double cost = static_cast<double>(transfer.bytes) * transfer.price_per_byte;
    transfer_seconds += duration;
    total_cost += cost;
    result["transfers"].push_back({
        {"id", input.at("runId").get<std::string>() + ":" + transfer.producer_id + ":" + transfer.consumer_id},
        {"executionRunId", input.at("runId")}, {"producerActivityId", transfer.producer_id},
        {"consumerActivityId", transfer.consumer_id}, {"sourceResourceId", transfer.source_resource_id},
        {"targetResourceId", transfer.target_resource_id}, {"bytes", transfer.bytes},
        {"startedAt", start}, {"finishedAt", finish}, {"durationSeconds", duration}, {"cost", cost},
    });
  }
  const double deadline = input.value("deadlineSeconds", 0.0);
  const double budget = input.value("budget", 0.0);
  const bool feasible = (deadline <= 0 || makespan <= deadline) && (budget <= 0 || total_cost <= budget);
  result["executed"] = {
      {"makespanSeconds", makespan}, {"cost", total_cost}, {"computeSeconds", compute},
      {"transferSeconds", transfer_seconds}, {"queueSeconds", queue}, {"interferenceSeconds", 0},
      {"overheadSeconds", overhead_seconds}, {"feasible", feasible},
  };
  return result;
}

int main(int argc, char** argv)
{
  try {
    const Arguments arguments = parse_arguments(argc, argv);
    const json input           = read_json(arguments.input);
    sg4::Engine engine(&argc, argv);
    engine.load_platform(arguments.platform);
    const auto hosts = engine.get_all_hosts();
    if (hosts.empty())
      throw std::runtime_error("platform has no hosts");
    auto tasks     = create_tasks(engine, input);
    auto transfers = create_transfers(input, tasks);
    run_simulation(engine, input, tasks, transfers);
    const json result = build_result(input, tasks, transfers);
    write_json(arguments.output, result);
    return 0;
  } catch (const std::exception& error) {
    std::cerr << "akoflow-simgrid-runner: " << error.what() << '\n';
    return 1;
  }
}

import json
import pygame

MARGIN = 40
SCALE = 4
SIDEBAR_WIDTH = 300

SECTORS_PATH = "data/interface/sectors.json"
DRONES_PATH = "data/interface/drones.json"
SENSORS_PATH = "data/interface/sensors.json"
REQUESTS_PATH = "data/interface/requests.json"

DRONE_IMAGE_PATH = "data/images/drone.webp"


# load_list Carrega uma lista a partir de um arquivo JSON e retorna lista vazia em caso de erro ou conteúdo nulo
def load_list(path):
    try:
        with open(path, "r", encoding="utf-8") as file:
            data = json.load(file)

        if data is None:
            return []

        return data

    except:
        return []


# load_world Carrega o estado atual do sistema a partir dos arquivos de Interface
def load_world():
    return {
        "sectors": load_list(SECTORS_PATH),
        "drones": load_list(DRONES_PATH),
        "sensors": load_list(SENSORS_PATH),
        "requests": load_list(REQUESTS_PATH),
    }


# get_world_limits Calcula os limites do mundo com base nos Setores para dimensionar a visualização
def get_world_limits(sectors):
    if not sectors:
        return 0, 300, 0, 100

    min_x = min(min(sector["left"], sector["right"]) for sector in sectors)
    max_x = max(max(sector["left"], sector["right"]) for sector in sectors)

    min_y = min(min(sector["top"], sector["bottom"]) for sector in sectors)
    max_y = max(max(sector["top"], sector["bottom"]) for sector in sectors)

    return min_x, max_x, min_y, max_y


# get_screen_size Converte os limites do mundo em dimensões de janela, reservando uma barra lateral para a fila
def get_screen_size(min_x, max_x, min_y, max_y):
    world_width = int((max_x - min_x) * SCALE + 2 * MARGIN)
    height = int((max_y - min_y) * SCALE + 2 * MARGIN)

    total_width = world_width + SIDEBAR_WIDTH
    return total_width, max(height, 400) # Garante uma altura mínima para o painel


# world_to_screen Converte coordenadas do mundo em coordenadas de tela considerando margem, escala e eixo Y invertido
def world_to_screen(x, y, min_x, max_y):
    screen_x = MARGIN + (x - min_x) * SCALE
    screen_y = MARGIN + (max_y - y) * SCALE

    return int(screen_x), int(screen_y)


# draw_sectors Desenha os retângulos dos Setores e seus identificadores
def draw_sectors(screen, font, sectors, min_x, max_y):
    for sector in sectors:
        left, top = world_to_screen(sector["left"], sector["top"], min_x, max_y)
        right, bottom = world_to_screen(sector["right"], sector["bottom"], min_x, max_y)

        rect_left = min(left, right)
        rect_top = min(top, bottom)
        rect_width = abs(right - left)
        rect_height = abs(bottom - top)

        rect = pygame.Rect(rect_left, rect_top, rect_width, rect_height)

        pygame.draw.rect(screen, (60, 60, 60), rect, 2)

        # Compatibilidade com chaves ID e id devido a diferenças de serialização
        text = font.render(f"Setor {sector.get('ID', sector.get('id', '?'))}", True, (200, 200, 200))
        screen.blit(text, (rect_left + 10, rect_top + 10))


# draw_sensors Desenha os Sensores como pontos e adiciona um marcador textual
def draw_sensors(screen, font, sensors, min_x, max_y):
    for sensor in sensors:
        x, y = world_to_screen(sensor["x"], sensor["y"], min_x, max_y)

        color = (0, 180, 255)
        radius = 6

        pygame.draw.circle(screen, color, (x, y), radius)

        text = font.render("S", True, (255, 255, 255))
        screen.blit(text, (x + 8, y - 8))


# draw_drones Desenha os Drones na tela usando imagem quando disponível ou círculos como fallback
def draw_drones(screen, font, drones, drone_image, min_x, max_y):
    for drone in drones:
        x, y = world_to_screen(drone["x"], drone["y"], min_x, max_y)

        if drone_image is not None:
            image_rect = drone_image.get_rect(center=(x, y))
            screen.blit(drone_image, image_rect)
        else:
            # Cores diferentes indicam ocupado e livre quando imagem não está disponível
            color = (255, 100, 100) if drone.get("is_busy") else (100, 255, 100)
            pygame.draw.circle(screen, color, (x, y), 12)

        text = font.render(f"D{drone.get('id', '?')}", True, (255, 255, 255))
        screen.blit(text, (x + 14, y - 8))


# draw_grid Desenha uma grade de referência no plano para facilitar leitura de posições
def draw_grid(screen, font, min_x, max_x, min_y, max_y, width, height):
    start_x = int(min_x)
    end_x = int(max_x)

    world_width = width - SIDEBAR_WIDTH

    for x in range(start_x, end_x + 1, 50):
        sx, _ = world_to_screen(x, min_y, min_x, max_y)
        pygame.draw.line(screen, (40, 40, 40), (sx, MARGIN), (sx, height - MARGIN), 1)
        text = font.render(str(x), True, (150, 150, 150))
        screen.blit(text, (sx - 10, height - MARGIN + 5))

    start_y = int(min_y)
    end_y = int(max_y)

    for y in range(start_y, end_y + 1, 20):
        _, sy = world_to_screen(min_x, y, min_x, max_y)
        pygame.draw.line(screen, (40, 40, 40), (MARGIN, sy), (world_width - MARGIN, sy), 1)
        text = font.render(str(y), True, (150, 150, 150))
        screen.blit(text, (5, sy - 8))


# draw_requests_panel Renderiza a barra lateral com a fila de Requisições, priorizando críticas e ordenando por Clock
def draw_requests_panel(screen, font, requests, width, height):
    panel_rect = pygame.Rect(width - SIDEBAR_WIDTH, 0, SIDEBAR_WIDTH, height)
    pygame.draw.rect(screen, (30, 30, 35), panel_rect)
    pygame.draw.line(screen, (80, 80, 80), (width - SIDEBAR_WIDTH, 0), (width - SIDEBAR_WIDTH, height), 2)

    title_font = pygame.font.SysFont("Arial", 20, bold=True)
    title = title_font.render("Fila de Requisições", True, (255, 255, 255))
    screen.blit(title, (width - SIDEBAR_WIDTH + 20, 20))

    y_offset = 60

    if not requests:
        empty_text = font.render("Nenhuma requisição ativa", True, (120, 120, 120))
        screen.blit(empty_text, (width - SIDEBAR_WIDTH + 20, y_offset))
        return

    requests_sorted = sorted(requests, key=lambda r: (-r.get('is_critical', 0), r.get('clock', 0)))

    for req in requests_sorted:
        sector_id = req.get("sector_id", "?")
        req_id = req.get("origin_id", "?")
        status = req.get("status", "UNKNOWN")
        is_critical = req.get("is_critical", False)

        color = (200, 200, 200)
        if status == "ATTENDING":
            color = (100, 255, 100)
        elif is_critical:
            color = (255, 100, 100)

        # Tradução de estados para exibição amigável
        status_pt = {
            "ATTENDING": "ATENDENDO",
            "PENDING": "PENDENTE",
            "COMPLETED": "CONCLUÍDO",
            "FAILED": "FALHOU",
            "UNKNOWN": "DESCONHECIDO"
        }.get(status, status)

        text_str = f"[ {sector_id} - {req_id} ]  {status_pt}"

        text = font.render(text_str, True, color)
        screen.blit(text, (width - SIDEBAR_WIDTH + 20, y_offset))

        y_offset += 25
        if y_offset > height - 30:
            text = font.render("...", True, (200, 200, 200))
            screen.blit(text, (width - SIDEBAR_WIDTH + 20, y_offset))
            break


# main Inicializa o Pygame, carrega o estado em loop e renderiza Setores, Sensores, Drones e a fila de Requisições
def main():
    pygame.init()

    world = load_world()
    sectors = world.get("sectors") or []

    min_x, max_x, min_y, max_y = get_world_limits(sectors)
    width, height = get_screen_size(min_x, max_x, min_y, max_y)

    screen = pygame.display.set_mode((width, height), pygame.RESIZABLE)
    pygame.display.set_caption("Monitoramento - Strait of Hormuz")

    font = pygame.font.SysFont("Arial", 16)
    clock = pygame.time.Clock()

    try:
        drone_image = pygame.image.load(DRONE_IMAGE_PATH).convert_alpha()
        drone_image = pygame.transform.scale(drone_image, (32, 32))
    except:
        drone_image = None

    running = True

    while running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False

        world = load_world()

        sectors = world.get("sectors") or []
        sensors = world.get("sensors") or []
        drones = world.get("drones") or []
        requests = world.get("requests") or []

        new_min_x, new_max_x, new_min_y, new_max_y = get_world_limits(sectors)
        new_width, new_height = get_screen_size(new_min_x, new_max_x, new_min_y, new_max_y)

        # Ajusta dinamicamente a janela caso o mundo mude de tamanho
        if new_width != width or new_height != height:
            min_x = new_min_x
            max_x = new_max_x
            min_y = new_min_y
            max_y = new_max_y

            width = new_width
            height = new_height

            screen = pygame.display.set_mode((width, height), pygame.RESIZABLE)

        screen.fill((20, 20, 20))

        draw_grid(screen, font, min_x, max_x, min_y, max_y, width, height)
        draw_sectors(screen, font, sectors, min_x, max_y)
        draw_sensors(screen, font, sensors, min_x, max_y)
        draw_drones(screen, font, drones, drone_image, min_x, max_y)
        draw_requests_panel(screen, font, requests, width, height)

        pygame.display.flip()
        clock.tick(30)

    pygame.quit()


if __name__ == "__main__":
    main()
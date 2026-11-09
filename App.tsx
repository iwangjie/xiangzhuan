import { mount, View } from "nsh";
import { MainWindow } from "./src/MainWindow";
import { RestWindows } from "./src/RestWindows";
import { Shell } from "./src/Shell";

function App() {
  return (
    <View class="flex-1 bg-background">
      <MainWindow />
      <RestWindows />
      <Shell />
    </View>
  );
}

mount(App);

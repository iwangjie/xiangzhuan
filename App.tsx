import { mount, View } from "nsh";
import { MainWindow } from "./src/MainWindow";
import { RestWindows } from "./src/RestWindows";
import { Shell } from "./src/Shell";

function App() {
  return (
    <View style={{ flex: 1, backgroundColor: "transparent" }}>
      <MainWindow />
      <RestWindows />
      <Shell />
    </View>
  );
}

mount(App);

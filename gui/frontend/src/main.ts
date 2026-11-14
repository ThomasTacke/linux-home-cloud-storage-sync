import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { followSystemTheme } from "$lib/theme";

followSystemTheme();
mount(App, { target: document.getElementById("app")! });
